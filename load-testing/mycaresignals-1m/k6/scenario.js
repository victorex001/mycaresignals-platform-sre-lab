import http from 'k6/http';
import crypto from 'k6/crypto';
import encoding from 'k6/encoding';
import exec from 'k6/execution';
import { check, sleep } from 'k6';
import { Counter, Rate, Trend } from 'k6/metrics';

const baseURL = (__ENV.BASE_URL || '').replace(/\/$/, '');
const jwtSecret = __ENV.DEV_JWT_HMAC_SECRET || '';
const issuer = __ENV.DEV_JWT_ISSUER || 'https://sre-lab.invalid/mycaresignals';
const audience = __ENV.DEV_JWT_AUDIENCE || 'mycaresignals-sre-lab';
const profile = __ENV.PROFILE || 'smoke';
const shardCount = Number(__ENV.SHARD_COUNT || '1');
const shardIndex = Number(__ENV.SHARD_INDEX || '0');
const userPopulation = Number(__ENV.USER_POPULATION || '1000000');
const homeCount = Number(__ENV.HOME_COUNT || '10000');

if (!baseURL || jwtSecret.length < 32) {
  throw new Error('BASE_URL and a DEV_JWT_HMAC_SECRET of at least 32 characters are required');
}
if (shardCount !== 1 || shardIndex !== 0) {
  throw new Error('The USD 50 first exercise requires one shard with index zero');
}

// Dataset size and concurrency are separate controls. Larger runs need a new review.
const stageProfiles = {
  smoke: [
    { duration: '1m', target: 10 },
    { duration: '3m', target: 10 },
    { duration: '1m', target: 0 },
  ],
  baseline: [
    { duration: '3m', target: 100 },
    { duration: '9m', target: 100 },
    { duration: '3m', target: 0 },
  ],
};

if (!stageProfiles[profile]) {
  throw new Error(`Unknown PROFILE ${profile}`);
}

function shardStages(stages) {
  return stages.map((stage) => ({
    duration: stage.duration,
    target: stage.target === 0 ? 0 : Math.max(1, Math.ceil(stage.target / shardCount)),
  }));
}

export const options = {
  scenarios: {
    staff_journeys: {
      executor: 'ramping-vus',
      startVUs: 0,
      stages: shardStages(stageProfiles[profile]),
      gracefulRampDown: '30s',
      gracefulStop: '30s',
      tags: { profile },
    },
  },
  thresholds: {
    business_latency: ['p(95)<500'],
    unexpected_response: ['rate<0.01'],
    audit_write_failed: ['rate<0.01'],
    tenant_denial_bypass: [{ threshold: 'count==0', abortOnFail: true }],
    checks: ['rate>0.99'],
  },
  discardResponseBodies: true,
  noConnectionReuse: false,
  userAgent: `myCareSignals-sre-lab/${profile}`,
};

const businessLatency = new Trend('business_latency', true);
const unexpectedResponse = new Rate('unexpected_response');
const auditWriteFailed = new Rate('audit_write_failed');
const tenantDenialBypass = new Counter('tenant_denial_bypass');
const tokenCache = {};

function jwt(subject) {
  if (tokenCache[subject]) return tokenCache[subject];

  const now = Math.floor(Date.now() / 1000);
  const header = encoding.b64encode(JSON.stringify({ alg: 'HS256', typ: 'JWT' }), 'rawurl');
  const payload = encoding.b64encode(JSON.stringify({
    iss: issuer,
    aud: audience,
    sub: subject,
    iat: now,
    exp: now + 43200,
  }), 'rawurl');
  const signingInput = `${header}.${payload}`;
  const signature = crypto.hmac('sha256', jwtSecret, signingInput, 'base64rawurl');
  tokenCache[subject] = `${signingInput}.${signature}`;
  return tokenCache[subject];
}

function request(method, path, token, expectedStatus, body, flow) {
  const response = http.request(method, `${baseURL}${path}`, body || null, {
    headers: {
      Authorization: `Bearer ${token}`,
      'Content-Type': 'application/json',
    },
    tags: { flow, expected_status: String(expectedStatus) },
    timeout: '10s',
  });

  const accepted = check(response, {
    [`${flow} returned ${expectedStatus}`]: (result) => result.status === expectedStatus,
  });
  unexpectedResponse.add(!accepted, { flow });
  businessLatency.add(response.timings.duration, { flow });
  return response;
}

export default function () {
  const sequence = exec.scenario.iterationInTest + exec.vu.idInTest + (shardIndex * 104729);
  const userNumber = (sequence % userPopulation) + 1;
  const homeNumber = ((userNumber - 1) % homeCount) + 1;
  const homeID = 1000000 + homeNumber;
  const childID = 2000000 + homeNumber;
  const otherChildID = 2000000 + (homeNumber % homeCount) + 1;
  const actorToken = jwt(`sre-user-${userNumber}`);
  const managerToken = jwt(`sre-user-${homeNumber}`);
  const choice = Math.random();

  if (choice < 0.45) {
    request('GET', `/api/te/v1/homes/${homeID}/children`, actorToken, 200, null, 'home_children');
  } else if (choice < 0.70) {
    request('GET', `/api/te/v1/children/${childID}/care-overview`, actorToken, 200, null, 'care_overview');
  } else if (choice < 0.87) {
    request('GET', `/api/te/v1/children/${childID}/goals`, actorToken, 200, null, 'goals_read');
  } else if (choice < 0.97) {
    const response = request(
      'POST',
      `/api/te/v1/children/${childID}/goals`,
      managerToken,
      201,
      JSON.stringify({
        goal_type: 'therapeutic',
        title: `Synthetic exercise goal ${sequence}`,
        domain: 'emotional',
        description: 'Synthetic SRE load-test record; no personal data.',
        status: 'active',
      }),
      'goal_write',
    );
    auditWriteFailed.add(response.status !== 201);
  } else {
    const response = request(
      'GET',
      `/api/te/v1/children/${otherChildID}/goals`,
      actorToken,
      403,
      null,
      'cross_tenant_denial',
    );
    if (response.status >= 200 && response.status < 300) tenantDenialBypass.add(1);
  }

  sleep(0.5 + Math.random() * 1.5);
}

export function handleSummary(data) {
  return {
    stdout: JSON.stringify({
      test: 'mycaresignals-1m',
      profile,
      shard_index: shardIndex,
      shard_count: shardCount,
      generated_at: new Date().toISOString(),
      metrics: data.metrics,
    }),
  };
}
