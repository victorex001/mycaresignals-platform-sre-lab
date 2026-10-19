const API_BASE = import.meta.env.VITE_API_BASE_URL ?? "/api/te";
const AUTH_MODE = import.meta.env.VITE_AUTH_MODE ?? (import.meta.env.DEV ? "development" : "cognito");
const COGNITO_DOMAIN = (import.meta.env.VITE_COGNITO_DOMAIN ?? "").replace(/\/$/, "");
const COGNITO_CLIENT_ID = import.meta.env.VITE_COGNITO_CLIENT_ID ?? "";
const COGNITO_REDIRECT_URI = import.meta.env.VITE_COGNITO_REDIRECT_URI ?? (typeof window !== "undefined" ? window.location.origin : "");
const COGNITO_LOGOUT_URI = import.meta.env.VITE_COGNITO_LOGOUT_URI ?? COGNITO_REDIRECT_URI;

export const DEV_TOKEN_STORAGE_KEY = "mycaresignals_dev_jwt";
export const DEV_AUTH_CHANGED_EVENT = "mycaresignals:dev-auth-changed";
export const INVALID_DEV_TOKEN_MESSAGE =
  "Development token is not a valid JWT. Paste the raw token only.";
export const AUTH_CHANGED_EVENT = "mycaresignals:auth-changed";
export const COGNITO_SESSION_STORAGE_KEY = "mycaresignals_cognito_session";
const PKCE_VERIFIER_STORAGE_KEY = "mycaresignals_pkce_verifier";
const OAUTH_STATE_STORAGE_KEY = "mycaresignals_oauth_state";

const compactJWTPattern =
  /^[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+$/;

let pendingDevAuthError: string | null = null;

export class ApiError extends Error {
  readonly status: number;

  constructor(message: string, status: number) {
    super(message);
    this.name = "ApiError";
    this.status = status;
  }
}

const devStorageAvailable = () =>
  import.meta.env.DEV && typeof window !== "undefined";

const browserStorageAvailable = () => typeof window !== "undefined";

export function productionAuthEnabled(): boolean {
  return AUTH_MODE === "cognito";
}

const notifyDevAuthChanged = () => {
  if (typeof window !== "undefined") {
    window.dispatchEvent(new Event(DEV_AUTH_CHANGED_EVENT));
  }
};

function normalizeDevToken(value: string): string | null {
  const token = value.trim();
  const containsNonASCII = Array.from(token).some(
    (character) => character.charCodeAt(0) > 127,
  );
  if (
    !token ||
    /^Bearer(?:\s|$)/i.test(token) ||
    /["'`]/.test(token) ||
    containsNonASCII ||
    !compactJWTPattern.test(token)
  ) {
    return null;
  }
  return token;
}

function discardInvalidStoredToken(): void {
  try {
    window.sessionStorage.removeItem(DEV_TOKEN_STORAGE_KEY);
  } catch {
    // Storage can be unavailable in restricted browser contexts.
  } finally {
    pendingDevAuthError = INVALID_DEV_TOKEN_MESSAGE;
    notifyDevAuthChanged();
  }
}

export function consumeDevAuthError(): string | null {
  const message = pendingDevAuthError;
  pendingDevAuthError = null;
  return message;
}

export function getDevToken(): string | null {
  if (!devStorageAvailable()) return null;

  try {
    const storedToken = window.sessionStorage.getItem(DEV_TOKEN_STORAGE_KEY);
    if (storedToken === null) return null;

    const token = normalizeDevToken(storedToken);
    if (!token) {
      discardInvalidStoredToken();
      return null;
    }
    return token;
  } catch {
    return null;
  }
}

export function setDevToken(token: string): boolean {
  if (!devStorageAvailable()) return false;

  const normalized = normalizeDevToken(token);
  if (!normalized) {
    clearDevToken();
    return false;
  }

  try {
    window.sessionStorage.setItem(DEV_TOKEN_STORAGE_KEY, normalized);
    pendingDevAuthError = null;
    notifyDevAuthChanged();
    return true;
  } catch {
    // Storage can be unavailable in restricted browser contexts. Keep auth
    // fail-closed and leave the development access gate visible.
    return false;
  }
}

export function clearDevToken(): void {
  if (!devStorageAvailable()) return;

  try {
    window.sessionStorage.removeItem(DEV_TOKEN_STORAGE_KEY);
  } catch {
    // Keep request handling fail-closed if browser storage is unavailable.
  } finally {
    pendingDevAuthError = null;
    notifyDevAuthChanged();
  }
}

type CognitoSession = {
  accessToken: string;
  idToken?: string;
  expiresAt: number;
};

function notifyAuthChanged() {
  if (typeof window !== "undefined") {
    window.dispatchEvent(new Event(AUTH_CHANGED_EVENT));
  }
}

function getCognitoSession(): CognitoSession | null {
  if (!browserStorageAvailable()) return null;
  try {
    const raw = window.sessionStorage.getItem(COGNITO_SESSION_STORAGE_KEY);
    if (!raw) return null;
    const session = JSON.parse(raw) as Partial<CognitoSession>;
    if (typeof session.accessToken !== "string" || typeof session.expiresAt !== "number") {
      clearCognitoSession();
      return null;
    }
    if (session.expiresAt <= Date.now() + 30_000) {
      clearCognitoSession();
      return null;
    }
    return session as CognitoSession;
  } catch {
    clearCognitoSession();
    return null;
  }
}

export function hasProductionSession(): boolean {
  return getCognitoSession() !== null;
}

export function clearCognitoSession(): void {
  if (!browserStorageAvailable()) return;
  try {
    window.sessionStorage.removeItem(COGNITO_SESSION_STORAGE_KEY);
    window.sessionStorage.removeItem(PKCE_VERIFIER_STORAGE_KEY);
    window.sessionStorage.removeItem(OAUTH_STATE_STORAGE_KEY);
  } catch {
    // Keep auth fail-closed if browser storage is unavailable.
  } finally {
    notifyAuthChanged();
  }
}

function requireCognitoConfig() {
  if (!COGNITO_DOMAIN || !COGNITO_CLIENT_ID || !COGNITO_REDIRECT_URI) {
    throw new ApiError("Staff sign in is not configured.", 500);
  }
}

function base64URL(bytes: Uint8Array): string {
  let binary = "";
  bytes.forEach((byte) => { binary += String.fromCharCode(byte); });
  return btoa(binary).replace(/\+/g, "-").replace(/\//g, "_").replace(/=+$/, "");
}

async function sha256Base64URL(value: string): Promise<string> {
  const digest = await crypto.subtle.digest("SHA-256", new TextEncoder().encode(value));
  return base64URL(new Uint8Array(digest));
}

function randomBase64URL(byteCount: number): string {
  const bytes = new Uint8Array(byteCount);
  crypto.getRandomValues(bytes);
  return base64URL(bytes);
}

export async function startStaffSignIn(): Promise<void> {
  requireCognitoConfig();
  const verifier = randomBase64URL(64);
  const state = randomBase64URL(24);
  window.sessionStorage.setItem(PKCE_VERIFIER_STORAGE_KEY, verifier);
  window.sessionStorage.setItem(OAUTH_STATE_STORAGE_KEY, state);
  const challenge = await sha256Base64URL(verifier);
  const params = new URLSearchParams({
    client_id: COGNITO_CLIENT_ID,
    response_type: "code",
    scope: "openid email profile",
    redirect_uri: COGNITO_REDIRECT_URI,
    code_challenge_method: "S256",
    code_challenge: challenge,
    state,
  });
  window.location.assign(`${COGNITO_DOMAIN}/oauth2/authorize?${params.toString()}`);
}

export async function completeStaffSignInFromURL(): Promise<boolean> {
  if (!productionAuthEnabled() || !browserStorageAvailable()) return false;
  const url = new URL(window.location.href);
  const code = url.searchParams.get("code");
  const state = url.searchParams.get("state");
  if (!code) return hasProductionSession();
  requireCognitoConfig();
  const expectedState = window.sessionStorage.getItem(OAUTH_STATE_STORAGE_KEY);
  const verifier = window.sessionStorage.getItem(PKCE_VERIFIER_STORAGE_KEY);
  if (!state || state !== expectedState || !verifier) {
    clearCognitoSession();
    throw new ApiError("Staff sign in could not be completed.", 401);
  }
  const body = new URLSearchParams({
    grant_type: "authorization_code",
    client_id: COGNITO_CLIENT_ID,
    code,
    redirect_uri: COGNITO_REDIRECT_URI,
    code_verifier: verifier,
  });
  const response = await fetch(`${COGNITO_DOMAIN}/oauth2/token`, {
    method: "POST",
    headers: { "Content-Type": "application/x-www-form-urlencoded" },
    body,
  });
  if (!response.ok) {
    clearCognitoSession();
    throw new ApiError("Staff sign in was rejected.", response.status);
  }
  const tokenResponse = await response.json() as { access_token?: unknown; id_token?: unknown; expires_in?: unknown };
  if (typeof tokenResponse.access_token !== "string" || typeof tokenResponse.expires_in !== "number") {
    clearCognitoSession();
    throw new ApiError("Staff sign in returned an invalid session.", 401);
  }
  const session: CognitoSession = {
    accessToken: tokenResponse.access_token,
    idToken: typeof tokenResponse.id_token === "string" ? tokenResponse.id_token : undefined,
    expiresAt: Date.now() + tokenResponse.expires_in * 1000,
  };
  window.sessionStorage.setItem(COGNITO_SESSION_STORAGE_KEY, JSON.stringify(session));
  window.sessionStorage.removeItem(PKCE_VERIFIER_STORAGE_KEY);
  window.sessionStorage.removeItem(OAUTH_STATE_STORAGE_KEY);
  url.searchParams.delete("code");
  url.searchParams.delete("state");
  window.history.replaceState({}, document.title, url.pathname + url.search + url.hash);
  notifyAuthChanged();
  return true;
}

export function signOutStaff(): void {
  clearCognitoSession();
  if (!productionAuthEnabled() || !COGNITO_DOMAIN || !COGNITO_CLIENT_ID) return;
  const params = new URLSearchParams({
    client_id: COGNITO_CLIENT_ID,
    logout_uri: COGNITO_LOGOUT_URI,
  });
  window.location.assign(`${COGNITO_DOMAIN}/logout?${params.toString()}`);
}

export async function enterLocalManagerDevelopmentAccess(): Promise<boolean> {
  if (!devStorageAvailable()) return false;

  const response = await fetch(resolveApiURL("/internal/dev/local-manager-token"), {
    method: "POST",
  });
  if (!response.ok) {
    throw new ApiError(
      response.status === 404
        ? "Local manager access is not available in this environment."
        : "Local manager access could not be started.",
      response.status,
    );
  }
  const payload = await response.json() as { token?: unknown };
  if (typeof payload.token !== "string" || !setDevToken(payload.token)) {
    throw new ApiError(INVALID_DEV_TOKEN_MESSAGE, 401);
  }
  return true;
}

function resolveApiURL(path: string): string {
  const base = API_BASE.replace(/\/$/, "");
  if (/^https?:\/\//i.test(path)) {
    if (path === base || path.startsWith(`${base}/`)) return path;
    throw new ApiError("Invalid API URL.", 0);
  }
  if (path === base || path.startsWith(`${base}/`)) return path;
  return `${base}/${path.replace(/^\/+/, "")}`;
}

export async function apiFetch(
  path: string,
  options: RequestInit = {},
): Promise<Response> {
  const productionSession = productionAuthEnabled() ? getCognitoSession() : null;
  const token = productionSession?.accessToken ?? getDevToken();
  if (!token && pendingDevAuthError) {
    throw new ApiError(INVALID_DEV_TOKEN_MESSAGE, 401);
  }

  const headers = new Headers(options.headers);
  if (token) {
    headers.set("Authorization", `Bearer ${token}`);
  }

  const response = await fetch(resolveApiURL(path), {
    ...options,
    headers,
  });

  if (response.status === 401) {
    if (productionAuthEnabled()) clearCognitoSession();
    else clearDevToken();
    throw new ApiError(
      import.meta.env.DEV
        ? "Development access is required."
        : "Authentication is required.",
      401,
    );
  }
  if (response.status === 403) {
    throw new ApiError("Access denied.", 403);
  }

  return response;
}
