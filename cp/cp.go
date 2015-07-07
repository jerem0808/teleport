package cp

import (
	"archive/tar"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"time"

	"code.google.com/p/go-uuid/uuid"
	"github.com/gravitational/teleport/auth"
	"github.com/gravitational/teleport/backend"
	"github.com/gravitational/teleport/sshutils"
	"github.com/gravitational/teleport/sshutils/scp"
	"github.com/gravitational/teleport/utils"

	"github.com/gravitational/teleport/Godeps/_workspace/src/github.com/gravitational/form"
	"github.com/gravitational/teleport/Godeps/_workspace/src/github.com/gravitational/roundtrip"
	"github.com/gravitational/teleport/Godeps/_workspace/src/github.com/julienschmidt/httprouter"
	"github.com/gravitational/teleport/Godeps/_workspace/src/github.com/mailgun/log"
	"github.com/mailgun/ttlmap"
)

// cpHandler implements methods for control panel
type cpHandler struct {
	httprouter.Router
	host        string
	authServers []utils.NetAddr
	sessions    *ttlmap.TtlMap
}

func newCPHandler(host string, auth []utils.NetAddr, assetsDir string) *cpHandler {
	initTemplates(assetsDir)

	m, err := ttlmap.NewMap(1024)
	if err != nil {
		panic(err)
	}

	h := &cpHandler{
		authServers: auth,
		host:        host,
		sessions:    m,
	}
	h.GET("/login", h.login)
	h.GET("/logout", h.logout)
	h.POST("/auth", h.authForm)

	// WEB views
	h.GET("/", h.needsAuth(h.keysIndex))
	h.GET("/keys", h.needsAuth(h.keysIndex))
	h.GET("/events", h.needsAuth(h.eventsIndex))
	h.GET("/webtuns", h.needsAuth(h.webTunsIndex))
	h.GET("/servers", h.needsAuth(h.serversIndex))
	h.GET("/sessions", h.needsAuth(h.sessionsIndex))
	h.POST("/sessions", h.needsAuth(h.newSession))
	h.GET("/sessions/:id", h.needsAuth(h.sessionIndex))
	h.POST("/servers/:id/files", h.needsAuth(h.uploadFile))
	h.GET("/servers/:id/ls", h.needsAuth(h.ls))
	h.GET("/servers/:id/download", h.needsAuth(h.downloadFiles))

	// JSON API methods

	// Key Management
	h.GET("/api/keys", h.needsAuth(h.getKeys))
	h.POST("/api/keys", h.needsAuth(h.postKey))
	h.DELETE("/api/keys/:key", h.needsAuth(h.deleteKey))

	// Event log
	h.GET("/api/events", h.needsAuth(h.getEvents))

	// Web tunnels
	h.GET("/api/tunnels/web", h.needsAuth(h.getWebTuns))
	h.POST("/api/tunnels/web", h.needsAuth(h.upsertWebTun))
	h.GET("/api/tunnels/web/:prefix", h.needsAuth(h.getWebTun))
	h.DELETE("/api/tunnels/web/:prefix", h.needsAuth(h.deleteWebTun))

	// Remote access to SSH server
	h.GET("/api/ssh/connect/:server/sessions/:sid", h.needsAuth(h.connect))

	// Operations with servers
	h.GET("/api/servers", h.needsAuth(h.getServers))

	// Static assets
	h.Handler("GET", "/static/*filepath",
		http.FileServer(http.Dir(filepath.Join(assetsDir, "assets"))))

	// Operations with sessions
	h.GET("/api/sessions", h.needsAuth(h.getSessions))
	h.GET("/api/sessions/:id", h.needsAuth(h.getSession))

	return h
}

func (s *cpHandler) ls(w http.ResponseWriter, r *http.Request, p httprouter.Params, c *ctx) {
	root := r.URL.Query().Get("node")
	log.Infof("!!!! LS: root: %v", root)

	addr := p[0].Value

	up, err := c.connectUpstream(addr)
	if err != nil {
		log.Errorf("file err: %v", err)
		replyErr(w, http.StatusInternalServerError, err)
		return
	}

	session := up.GetSession()

	stdout, err := session.StdoutPipe()
	if err != nil {
		log.Errorf("file err: %v", err)
		replyErr(w, http.StatusInternalServerError, err)
		return
	}

	if err := session.RequestSubsystem(fmt.Sprintf("ls:%v", root)); err != nil {
		log.Errorf("file err: %v", err)
		replyErr(w, http.StatusInternalServerError, err)
		return
	}

	out, err := ioutil.ReadAll(stdout)
	if err != nil {
		log.Errorf("file err: %v", err)
		replyErr(w, http.StatusInternalServerError, err)
		return
	}

	var nodes []utils.FileNode
	if err := json.Unmarshal(out, &nodes); err != nil {
		log.Errorf("file err: %v", err)
		replyErr(w, http.StatusInternalServerError, err)
		return
	}

	jsnodes := make([]interface{}, len(nodes))
	for i, n := range nodes {
		var icon string
		if n.Dir {
			icon = "fa fa-folder"
		} else {
			icon = "fa fa-file-code-o"
		}
		jsnodes[i] = jsNode{
			ID:       filepath.Join(n.Parent, n.Name),
			Text:     n.Name,
			Children: n.Dir,
			Icon:     icon,
		}

	}
	roundtrip.ReplyJSON(w, http.StatusOK, jsnodes)
}

func (s *cpHandler) downloadFiles(w http.ResponseWriter, r *http.Request, p httprouter.Params, c *ctx) {

	addr := p[0].Value

	files := r.URL.Query()["path"]
	if len(files) == 0 {
		replyErr(w, http.StatusInternalServerError, fmt.Errorf("need some files"))
		return
	}

	dir, err := ioutil.TempDir("", "test")
	if err != nil {
		log.Errorf("file err: %v", err)
		replyErr(w, http.StatusInternalServerError, err)
		return
	}
	defer func() {
		if err := os.RemoveAll(dir); err != nil {
			log.Infof("failed to remove temp file")
		}
	}()

	for _, p := range files {
		target := filepath.Join(dir, filepath.Dir(p))
		if err := os.MkdirAll(target, 0755); err != nil {
			log.Errorf("file err: %v", err)
			replyErr(w, http.StatusInternalServerError, err)
			return
		}

		up, err := c.connectUpstream(addr)
		if err != nil {
			log.Errorf("file err: %v", err)
			replyErr(w, http.StatusInternalServerError, err)
			return
		}

		rw, err := up.CommandRW(fmt.Sprintf("scp -v -f %v", p))
		uploader, err := scp.New(scp.Command{Sink: true, Target: dir})
		if err != nil {
			log.Errorf("file err: %v", err)
			replyErr(w, http.StatusInternalServerError, err)
			return
		}

		if err := uploader.Serve(rw); err != nil {
			log.Errorf("file err: %v", err)
			replyErr(w, http.StatusInternalServerError, err)
			return
		}
		if err := up.Close(); err != nil && err != io.EOF {
			log.Errorf("file err: %v", err)
			replyErr(w, http.StatusInternalServerError, err)
			return
		}
	}

	ck := &http.Cookie{
		Domain: fmt.Sprintf(".%v", s.host),
		Name:   "fileDownload",
		Value:  "true",
		Path:   "/",
	}
	http.SetCookie(w, ck)
	w.Header().Set("Content-Disposition", "attachment; filename=download.tar")
	writeArchive(dir, w)
}

func (s *cpHandler) uploadFile(w http.ResponseWriter, r *http.Request, _ httprouter.Params, c *ctx) {
	file, fh, err := r.FormFile("file")
	if err != nil {
		log.Errorf("file err: %v", err)
		replyErr(w, http.StatusInternalServerError, err)
		return
	}
	defer file.Close()

	path, addr := r.Form.Get("path"), r.Form.Get("addr")

	up, err := c.connectUpstream(addr)
	if err != nil {
		log.Errorf("file err: %v", err)
		replyErr(w, http.StatusInternalServerError, err)
		return
	}

	dir, err := ioutil.TempDir("", "test")
	if err != nil {
		log.Errorf("file err: %v", err)
		replyErr(w, http.StatusInternalServerError, err)
		return
	}
	fpath := filepath.Join(dir, fh.Filename)

	f, err := os.Create(fpath)
	if err != nil {
		log.Errorf("file err: %v", err)
		replyErr(w, http.StatusInternalServerError, err)
		return
	}
	if _, err := io.Copy(f, file); err != nil {
		log.Errorf("file err: %v", err)
		replyErr(w, http.StatusInternalServerError, err)
		return
	}
	if err := f.Close(); err != nil {
		log.Errorf("file err: %v", err)
		replyErr(w, http.StatusInternalServerError, err)
		return
	}
	defer func() {
		if err := os.RemoveAll(dir); err != nil {
			log.Infof("failed to remove temp file")
		}
	}()

	log.Infof("!!!!! 0 I am here")
	rw, err := up.CommandRW(fmt.Sprintf("scp -v -t %v", path))
	log.Infof("!!!!! 0 I am here 2")
	uploader, err := scp.New(scp.Command{Source: true, Target: f.Name()})
	if err != nil {
		log.Errorf("file err: %v", err)
		replyErr(w, http.StatusInternalServerError, err)
		return
	}

	if err := uploader.Serve(rw); err != nil {
		log.Errorf("file err: %v", err)
		replyErr(w, http.StatusInternalServerError, err)
		return
	}
	if err := up.Close(); err != nil {
		log.Errorf("file err: %v", err)
		replyErr(w, http.StatusInternalServerError, err)
		return
	}
	log.Infof("%v uploaded", fh.Filename)
	res := map[string]interface{}{
		"result": map[string]interface{}{
			"name": fh.Filename,
		},
	}
	roundtrip.ReplyJSON(w, http.StatusOK, res)
}

func (s *cpHandler) getSessions(w http.ResponseWriter, r *http.Request, _ httprouter.Params, c *ctx) {
	ses, err := c.clt.GetSessions()
	if err != nil {
		log.Errorf("failed to retrieve sessions: %v", err)
		replyErr(w, http.StatusInternalServerError, err)
		return
	}
	roundtrip.ReplyJSON(w, http.StatusOK, ses)
}

func (s *cpHandler) getSession(w http.ResponseWriter, r *http.Request, p httprouter.Params, c *ctx) {
	ses, err := c.clt.GetSession(p[0].Value)
	if err != nil {
		if !backend.IsNotFound(err) {
			log.Errorf("failed to retrieve session: %v", err)
			replyErr(w, http.StatusInternalServerError, err)
			return
		}
		if err = c.clt.UpsertSession(p[0].Value, 60*time.Second); err != nil {
			log.Errorf("failed to upsert session: %v", err)
			replyErr(w, http.StatusInternalServerError, err)
			return
		}
		if ses, err = c.clt.GetSession(p[0].Value); err != nil {
			log.Errorf("failed to upsert session: %v", err)
			replyErr(w, http.StatusInternalServerError, err)
			return
		}
	}
	srvs, err := c.clt.GetServers()
	if err != nil {
		log.Errorf("failed to retrieve servers: %v", err)
		replyErr(w, http.StatusInternalServerError, err)
		return
	}
	roundtrip.ReplyJSON(w, http.StatusOK,
		map[string]interface{}{
			"session": ses,
			"servers": srvs,
		})
}

func (s *cpHandler) getServers(w http.ResponseWriter, r *http.Request, _ httprouter.Params, c *ctx) {
	servers, err := c.clt.GetServers()
	if err != nil {
		log.Errorf("failed to retrieve servers: %v")
		replyErr(w, http.StatusInternalServerError, err)
		return
	}
	roundtrip.ReplyJSON(w, http.StatusOK, servers)
}

func (s *cpHandler) connect(w http.ResponseWriter, r *http.Request, p httprouter.Params, c *ctx) {
	log.Infof("connect request authorized to: %v", p[0].Value)
	ws := wsHandler{
		authServers: s.authServers,
		ctx:         c,
		addr:        p[0].Value,
		sid:         p[1].Value,
	}
	defer ws.Close()
	ws.Handler().ServeHTTP(w, r)
}

func (s *cpHandler) getWebTun(w http.ResponseWriter, r *http.Request, p httprouter.Params, c *ctx) {
	tun, err := c.clt.GetWebTun(p[0].Value)
	if err != nil {
		replyErr(w, http.StatusInternalServerError, err)
		return
	}
	roundtrip.ReplyJSON(w, http.StatusOK, tun)
}

func (s *cpHandler) deleteWebTun(w http.ResponseWriter, r *http.Request, p httprouter.Params, c *ctx) {
	if err := c.clt.DeleteWebTun(p[0].Value); err != nil {
		replyErr(w, http.StatusInternalServerError, err)
		return
	}
	roundtrip.ReplyJSON(w, http.StatusOK, "deleted")
}

func (s *cpHandler) getWebTuns(w http.ResponseWriter, r *http.Request, _ httprouter.Params, c *ctx) {
	tuns, err := c.clt.GetWebTuns()
	if err != nil {
		log.Errorf("failed to retrieve tunnels: %v")
		replyErr(w, http.StatusInternalServerError, err)
		return
	}
	roundtrip.ReplyJSON(w, http.StatusOK, tuns)
}

func (s *cpHandler) upsertWebTun(w http.ResponseWriter, r *http.Request, _ httprouter.Params, c *ctx) {
	var prefix, target, proxy string

	err := form.Parse(r,
		form.String("prefix", &prefix, form.Required()),
		form.String("target", &target, form.Required()),
		form.String("proxy", &proxy, form.Required()))
	if err != nil {
		log.Errorf("failed to parse form: %v", err)
		roundtrip.ReplyJSON(w, http.StatusBadRequest, message(err.Error()))
		return
	}
	wt, err := backend.NewWebTun(prefix, proxy, target)
	if err != nil {
		log.Errorf("failed to parse form: %v", err)
		roundtrip.ReplyJSON(w, http.StatusBadRequest, message(err.Error()))
		return
	}
	if err := c.clt.UpsertWebTun(*wt, 0); err != nil {
		log.Errorf("failed to upsert keys: %v", err)
		roundtrip.ReplyJSON(w, http.StatusBadRequest, err.Error())
		return
	}
	roundtrip.ReplyJSON(w, http.StatusOK, wt)
}

func (s *cpHandler) getEvents(w http.ResponseWriter, r *http.Request, _ httprouter.Params, c *ctx) {
	events, err := c.clt.GetEvents()
	if err != nil {
		log.Errorf("failed to retrieve events: %v")
		replyErr(w, http.StatusInternalServerError, err)
		return
	}
	roundtrip.ReplyJSON(w, http.StatusOK, events)
}

func (s *cpHandler) keysIndex(w http.ResponseWriter, r *http.Request, _ httprouter.Params, _ *ctx) {
	executeTemplate(w, "keys", nil)
}

func (s *cpHandler) eventsIndex(w http.ResponseWriter, r *http.Request, _ httprouter.Params, _ *ctx) {
	executeTemplate(w, "events", nil)
}

func (s *cpHandler) webTunsIndex(w http.ResponseWriter, r *http.Request, _ httprouter.Params, _ *ctx) {
	executeTemplate(w, "webtuns", nil)
}

func (s *cpHandler) serversIndex(w http.ResponseWriter, r *http.Request, _ httprouter.Params, _ *ctx) {
	executeTemplate(w, "servers", nil)
}

func (s *cpHandler) newSession(w http.ResponseWriter, r *http.Request, _ httprouter.Params, c *ctx) {
	var server string
	err := form.Parse(r, form.String("server", &server))
	if err != nil {
		log.Errorf("failed to parse form: %v", err)
		roundtrip.ReplyJSON(w, http.StatusBadRequest, message(err.Error()))
		return
	}
	sid := uuid.New()
	if err := c.clt.UpsertSession(sid, 30*time.Second); err != nil {
		replyErr(w, http.StatusInternalServerError, err)
		return
	}
	u := url.URL{
		Path: fmt.Sprintf("/sessions/%v", sid),
	}
	u.Query().Set("server", server)
	http.Redirect(w, r, u.String(), http.StatusFound)
}

func (s *cpHandler) sessionsIndex(w http.ResponseWriter, r *http.Request, _ httprouter.Params, _ *ctx) {
	executeTemplate(w, "sessions", nil)
}

func (s *cpHandler) sessionIndex(w http.ResponseWriter, r *http.Request, p httprouter.Params, _ *ctx) {
	executeTemplate(w, "session", map[string]interface{}{
		"SessionID":  p[0].Value,
		"ServerAddr": r.URL.Query().Get("server"),
	})
}

func (s *cpHandler) getKeys(w http.ResponseWriter, r *http.Request, _ httprouter.Params, c *ctx) {
	keys, err := c.clt.GetUserKeys(c.user)
	if err != nil {
		log.Errorf("failed to retrieve keys: %v")
		replyErr(w, http.StatusInternalServerError, err)
		return
	}
	roundtrip.ReplyJSON(w, http.StatusOK, keys)
}

func (s *cpHandler) postKey(w http.ResponseWriter, r *http.Request, _ httprouter.Params, c *ctx) {
	var key, id string

	err := form.Parse(r,
		form.String("value", &key, form.Required()),
		form.String("id", &id, form.Required()))
	if err != nil {
		log.Errorf("failed to parse form: %v", err)
		roundtrip.ReplyJSON(w, http.StatusBadRequest, message(err.Error()))
		return
	}
	cert, err := c.clt.UpsertUserKey(c.user, backend.AuthorizedKey{ID: id, Value: []byte(key)}, 0)
	if err != nil {
		log.Errorf("failed to upsert keys: %v", err)
		roundtrip.ReplyJSON(w, http.StatusBadRequest, message("invalid key format"))
		return
	}
	roundtrip.ReplyJSON(w, http.StatusOK, backend.AuthorizedKey{ID: key, Value: cert})
}

func (s *cpHandler) deleteKey(w http.ResponseWriter, r *http.Request, p httprouter.Params, c *ctx) {
	key := p[0].Value

	err := c.clt.DeleteUserKey(c.user, key)
	if err != nil {
		log.Errorf("failed to upsert keys: %v", err)
		roundtrip.ReplyJSON(w, http.StatusBadRequest, message("invalid key format"))
		return
	}
	roundtrip.ReplyJSON(w, http.StatusOK, message("key deleted"))
}

func (s *cpHandler) login(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	executeTemplate(w, "login", nil)
}

func (s *cpHandler) logout(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	if err := s.clearSession(w); err != nil {
		log.Errorf("failed to clear session: %v", err)
		replyErr(w, http.StatusInternalServerError, fmt.Errorf("failed to logout"))
		return
	}
	http.Redirect(w, r, "/login", http.StatusFound)
}

func (s *cpHandler) authForm(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
	var user, pass string

	err := form.Parse(r,
		form.String("username", &user, form.Required()),
		form.String("password", &pass, form.Required()))

	if err != nil {
		replyErr(w, http.StatusBadRequest, err)
		return
	}
	sid, err := s.auth(user, pass)
	if err != nil {
		log.Warningf("auth error: %v", err)
		http.Redirect(w, r, "/login", http.StatusFound)
		return
	}
	if err := s.setSession(w, user, sid); err != nil {
		replyErr(w, http.StatusInternalServerError, err)
		return
	}
	http.Redirect(w, r, "/", http.StatusFound)
}

func (s *cpHandler) auth(user, pass string) (string, error) {
	method, err := auth.NewWebPasswordAuth(user, []byte(pass))
	if err != nil {
		return "", err
	}
	clt, err := auth.NewTunClient(s.authServers[0], user, method)
	if err != nil {
		return "", err
	}
	return clt.SignIn(user, []byte(pass))
}

func (s *cpHandler) validateSession(user, sid string) (*ctx, error) {
	val, ok := s.sessions.Get(user + sid)
	if ok {
		log.Infof("retrieving session from cache")
		return val.(*ctx), nil
	}

	method, err := auth.NewWebSessionAuth(user, []byte(sid))
	if err != nil {
		return nil, err
	}
	clt, err := auth.NewTunClient(s.authServers[0], user, method)
	if err != nil {
		log.Infof("failed to connect: %v", clt, err)
		return nil, err
	}
	if _, err := clt.GetWebSession(user, sid); err != nil {
		log.Infof("session not found: %v", err)
		return nil, err
	}
	log.Infof("session validated")

	c := &ctx{
		clt:  clt,
		user: user,
		sid:  sid,
	}
	if err := s.sessions.Set(user+sid, c, 600); err != nil {
		log.Infof("something is wrong: %v", err)
		return nil, err
	}

	return c, nil
}

func (s *cpHandler) setSession(w http.ResponseWriter, user, sid string) error {
	d, err := encodeCookie(user, sid)
	if err != nil {
		return err
	}
	c := &http.Cookie{
		Domain: fmt.Sprintf(".%v", s.host),
		Name:   "session",
		Value:  d,
		Path:   "/",
	}
	http.SetCookie(w, c)
	return nil
}

func (s *cpHandler) clearSession(w http.ResponseWriter) error {
	http.SetCookie(w, &http.Cookie{
		Domain: fmt.Sprintf(".%v", s.host),
		Name:   "session",
		Value:  "",
		Path:   "/",
	})
	return nil
}

func (s *cpHandler) needsAuth(fn authHandle) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		cookie, err := r.Cookie("session")
		if err != nil {
			log.Infof("getting cookie: %v", err)
			http.Redirect(w, r, "/login", http.StatusFound)
			return
		}
		d, err := decodeCookie(cookie.Value)
		if err != nil {
			log.Warningf("failed to decode cookie '%v', err: %v", cookie.Value, err)
			http.Redirect(w, r, "/login", http.StatusFound)
			return
		}
		ctx, err := s.validateSession(d.User, d.SID)
		if err != nil {
			log.Warningf("failed to validate session: %v", err)
			http.Redirect(w, r, "/login", http.StatusFound)
			return
		}
		fn(w, r, p, ctx)
	}
	return nil
}

func replyErr(w http.ResponseWriter, code int, err error) {
	roundtrip.ReplyJSON(w, code, message(err.Error()))
}

func message(msg string) map[string]interface{} {
	return map[string]interface{}{"message": msg}
}

type cookie struct {
	User string
	SID  string
}

func encodeCookie(user, sid string) (string, error) {
	bytes, err := json.Marshal(cookie{User: user, SID: sid})
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

func decodeCookie(b string) (*cookie, error) {
	bytes, err := hex.DecodeString(b)
	if err != nil {
		return nil, err
	}
	var c *cookie
	if err := json.Unmarshal(bytes, &c); err != nil {
		return nil, err
	}
	return c, nil
}

func executeTemplate(w http.ResponseWriter, name string, data interface{}) {
	tpl, ok := templates[name]
	if !ok {
		replyErr(w, http.StatusInternalServerError, fmt.Errorf("template '%v' not found", name))
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := tpl.ExecuteTemplate(w, "base", data); err != nil {
		log.Errorf("Execute template: %v", err)
		replyErr(w, http.StatusInternalServerError, fmt.Errorf("internal render error"))
	}

}

type ctx struct {
	sid  string
	user string
	clt  *auth.TunClient
}

func (c *ctx) Close() error {
	if c.clt != nil {
		return c.clt.Close()
	}
	return nil
}

func (c *ctx) connectUpstream(addr string) (*sshutils.Upstream, error) {
	agent, err := c.clt.GetAgent()
	if err != nil {
		return nil, fmt.Errorf("failed to get agent: %v", err)
	}
	signers, err := agent.Signers()
	if err != nil {
		return nil, fmt.Errorf("no signers: %v", err)
	}
	return sshutils.DialUpstream(c.user, addr, signers)
}

type authHandle func(http.ResponseWriter, *http.Request, httprouter.Params, *ctx)

type jsNode struct {
	ID       string `json:"id"`
	Text     string `json:"text"`
	Children bool   `json:"children"`
	Icon     string `json:"icon"`
}

func writeArchive(root_directory string, w io.Writer) error {
	ar := tar.NewWriter(w)

	walkFn := func(path string, info os.FileInfo, err error) error {
		if info.Mode().IsDir() {
			return nil
		}
		// Because of scoping we can reference the external root_directory variable
		new_path := path[len(root_directory):]
		if len(new_path) == 0 {
			return nil
		}
		fr, err := os.Open(path)
		if err != nil {
			return err
		}
		defer fr.Close()

		if h, err := tar.FileInfoHeader(info, new_path); err != nil {
			return err
		} else {
			h.Name = new_path
			if err = ar.WriteHeader(h); err != nil {
				return err
			}
		}
		if length, err := io.Copy(ar, fr); err != nil {
			return err
		} else {
			fmt.Println(length)
		}
		return nil
	}

	return filepath.Walk(root_directory, walkFn)
}
