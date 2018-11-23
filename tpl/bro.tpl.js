// Available params:
// binary     :: bytes  - Base64-encoded API server binary
// port       :: int    - A port to listen on

function execute(tpl, params) {
  // Params type checking
  type.EnsureString("binary", params.binary);
  type.EnsureListOfNumbers("port", params.ports);

  // Create image
  var img = tpl.BuildImage("xenvman-tutorial");
  img.CopyDataToWorkspace("Dockerfile");

  // Extract server binary
  var bin = type.FromBase64("binary", params.binary);
  img.AddFileToWorkspace("bro", bin, 0755);

  // Create container
  var cont = img.NewContainer(service);
  cont.MountData("config.toml", "/config.toml", {"interpolate": true});

  if (type.IsDefined(params.port)) {
    cont.SetLabel("port", params.port);
  }
}