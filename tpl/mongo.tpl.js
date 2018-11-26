function execute(tpl, params) {
  var img = tpl.FetchImage("mongo:latest");
  var cont = img.NewContainer("mongo");

  cont.SetPorts(27017);
  cont.SetLabel("mongo", "true");
}