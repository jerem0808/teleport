{{ define "base" }}
<!DOCTYPE html>
<html>
<head>
    <meta charset="utf-8">
    <meta http-equiv="X-UA-Compatible" content="IE=edge">
    <meta name="viewport" content="width=device-width, initial-scale=1">
    <title>{{ template "title" . }}</title>

    <link href="/static/css/bootstrap.min.css" rel="stylesheet">
    <link href="/static/font-awesome/css/font-awesome.min.css" rel="stylesheet">
    <link href="/static/css/animate.css" rel="stylesheet">
    <link href="/static/css/style.min.css" rel="stylesheet">
    <link href="/static/css/plugins/chosen/chosen.css" rel="stylesheet">
    <link href="/static/css/plugins/toastr/toastr.min.css" rel="stylesheet">
    <link href="/static/css/plugins/fileupload/jquery.fileupload.css" rel="stylesheet">
    <link href="/static/css/plugins/fileupload/jquery.fileupload-ui.css" rel="stylesheet">
    <link href="/static/css/plugins/jsTree/style.min.css" rel="stylesheet">
</head>

<body>
    {{ template "body" . }}

    <!-- Mainly scripts -->
    <script src="/static/js/JSXTransformer.js"></script>
    <script src="/static/js/react.js"></script>

    <script src="/static/js/jquery-2.1.1.js"></script>
    <script src="/static/js/bootstrap.min.js"></script>
    <script src="/static/js/plugins/metisMenu/jquery.metisMenu.js"></script>
    <script src="/static/js/plugins/slimscroll/jquery.slimscroll.min.js"></script>
    
    <!-- Custom and plugin javascript -->
    <script src="/static/js/inspinia.js"></script>
    <script src="/static/js/plugins/pace/pace.min.js"></script>
    <script src="/static/js/plugins/chosen/chosen.jquery.js"></script>

    <script src="/static/js/jquery.ui.widget.js"></script>
    <script src="/static/js/jquery.iframe-transport.js"></script>
    <script src="/static/js/jquery.fileupload.js"></script>
    <script src="/static/js/jquery.fileupload-process.js"></script>
    <script src="/static/js/plugins/toastr/toastr.min.js"></script>
    <script src="/static/js/plugins/jsTree/jstree.min.js"></script>
    <script src="/static/js/plugins/download/jquery.fileDownload.js"></script>

    <!-- Gravity stuff -->
    <script src="/static/js/grv/lib.js"></script>    
    <script type="text/jsx" src="/static/js/grv/modal.jsx"></script>
    <script type="text/jsx" src="/static/js/grv/frame.js"></script>
    {{ template "script" . }}
</body>
</html>
{{ end }}

{{ define "script" }}{{ end }}
