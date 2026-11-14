window.onload = function () {
  window.ui = SwaggerUIBundle({
    url: "/api/v1/docs/openapi.yaml",
    dom_id: "#swagger-ui",
    deepLinking: true,
    validatorUrl: null,
    presets: [SwaggerUIBundle.presets.apis],
    layout: "BaseLayout",
  });
};
