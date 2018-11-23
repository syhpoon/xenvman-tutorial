function execute(tpl, params) {
  var img = tpl.FetchImage("mongo:latest");
  var cont = img.NewContainer("mongo");

  cont.SetLabel("mongo", "true");
}