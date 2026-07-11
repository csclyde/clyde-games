const http = require("node:http");
const https = require("node:https");

function postJson(url, payload) {
  const requestUrl = new URL(url);
  const body = JSON.stringify(payload);
  const transport = requestUrl.protocol === "https:" ? https : http;

  return new Promise((resolve, reject) => {
    const request = transport.request(
      requestUrl,
      {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
          "Content-Length": Buffer.byteLength(body),
        },
      },
      (response) => {
        let responseBody = "";
        response.setEncoding("utf8");
        response.on("data", (chunk) => {
          responseBody += chunk;
        });
        response.on("end", () => {
          if (response.statusCode < 200 || response.statusCode >= 300) {
            reject(new Error(`Feedback API returned ${response.statusCode}: ${responseBody}`));
            return;
          }

          resolve(responseBody ? JSON.parse(responseBody) : null);
        });
      },
    );

    request.on("error", reject);
    request.write(body);
    request.end();
  });
}

function createFeedbackClient(config) {
  return {
    submitFeedback(payload) {
      return postJson(config.feedbackApiUrl, payload);
    },
  };
}

module.exports = {
  createFeedbackClient,
  postJson,
};
