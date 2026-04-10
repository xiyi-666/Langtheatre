import http from "k6/http";
import { check, sleep } from "k6";

export const options = {
  vus: 5,
  duration: "30s",
  thresholds: {
    http_req_duration: ["p(95)<500"],
    http_req_failed: ["rate<0.05"],
  },
};

export default function () {
  const res = http.get(`${__ENV.BASE_URL || "http://localhost:8177"}/healthz`);
  check(res, {
    "status is 200 or 503": (r) => r.status === 200 || r.status === 503,
    "response has checks": (r) => r.body && r.body.includes("checks"),
  });
  sleep(1);
}
