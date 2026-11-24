const http = require("node:http");
const https = require("node:https");

function postJson(url, payload, token = "", method = "POST") {
  const requestUrl = new URL(url);
  const body = payload === undefined ? "" : JSON.stringify(payload);
  const transport = requestUrl.protocol === "https:" ? https : http;

  return new Promise((resolve, reject) => {
    const request = transport.request(
      requestUrl,
      {
        method,
        timeout: 15000,
        headers: {
          "Content-Type": "application/json",
          ...(token ? { Authorization: `Bearer ${token}` } : {}),
          "Content-Length": Buffer.byteLength(body),
        },
      },
      (response) => {
        let responseBody = "";
        response.setEncoding("utf8");
        response.on("data", (chunk) => {
          responseBody += chunk;
          if (responseBody.length > 2 * 1024 * 1024) { response.destroy(); reject(new Error("Feedback API response exceeds limit")); }
        });
        response.on("end", () => {
          if (response.statusCode < 200 || response.statusCode >= 300) {
            reject(new Error(`Feedback API returned ${response.statusCode}: ${responseBody}`));
            return;
          }

          try { resolve(responseBody ? JSON.parse(responseBody) : null); } catch { reject(new Error("Invalid feedback API JSON")); }
        });
      },
    );

    request.on("error", reject);
    request.on("timeout", () => request.destroy(new Error("Feedback API timed out")));
    request.write(body);
    request.end();
  });
}

function createFeedbackClient(config) {
  return {
    syncSources(){return postJson(`${config.feedbackInternalUrl}/sources`,undefined,config.feedbackIngestToken,"GET")},
    markResult(id,result){return postJson(`${config.feedbackInternalUrl}/messages/${id}/mark-result`,result,config.feedbackIngestToken)},
    submitSnapshot(snapshot) {
      if (!config.feedbackIngestToken || config.feedbackIngestToken.length < 32) throw new Error("Durable Discord intake token not configured");
      return postJson(`${config.feedbackInternalUrl}/manual`, snapshot, config.feedbackIngestToken);
    },
    intakeStatus(id) { return postJson(`${config.feedbackInternalUrl}/messages/${id}/status`, undefined, config.feedbackIngestToken, "GET"); },
    submitFeedback(payload) {
      return postJson(config.feedbackApiUrl, payload);
    },
  };
}

module.exports = {
  createFeedbackClient,
  postJson,
};
