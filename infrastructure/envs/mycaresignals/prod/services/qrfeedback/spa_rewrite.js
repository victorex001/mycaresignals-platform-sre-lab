function handler(event) {
  var request = event.request;
  var uri = request.uri;
  var finalSegment = uri.substring(uri.lastIndexOf("/") + 1);

  if (uri.indexOf("/api/") === 0) {
    return request;
  }

  // Route extensionless frontend navigation to the SPA entry point. This
  // function is associated only with the default S3 behavior, never /api/*.
  if (uri !== "/" && uri !== "/index.html" && finalSegment.indexOf(".") === -1) {
    request.uri = "/index.html";
  }

  return request;
}
