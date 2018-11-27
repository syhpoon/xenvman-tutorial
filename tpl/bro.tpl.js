// Available params:
// binary     :: string*  - Base64-encoded API server binary
// port       :: int      - A port to listen on

function execute(tpl, params) {
  // Params type checking
  type.EnsureString("binary", params.binary);
  type.EnsureNumber("port", params.port);

  // Create image
  var img = tpl.BuildImage("xenvman-tutorial");
  img.CopyDataToWorkspace("Dockerfile");

  // Extract server binary
  var bin = type.FromBase64("binary", params.binary);
  img.AddFileToWorkspace("bro", bin, 0755);

  // Create container
  var cont = img.NewContainer("bro");
  cont.MountData("config.toml", "/config.toml", {"interpolate": true});

  var port = 9999;

  if (type.IsDefined(params.port)) {
    port = params.port;
  }

  cont.SetPorts(port);
  cont.SetLabel("port", port);
  cont.SetLabel("bro", "true");

  tpl.AddReadinessCheck("http", {
    "url": fmt('http://{{.ExternalAddress}}:{{.ExposedContainerPort "bro" %v}}/',
               port),
    "codes": [200]
  });
}