// frontend/src/config/api.ts

// The QR API is served by the same CloudFront distribution under /api/*.
// Keeping this relative prevents either public QR hostname from depending on
// another hostname and keeps browser requests same-origin.
export const API_BASE_URL = "";

export async function postJson<T>(
  path: string,
  body: unknown
): Promise<T> {
  const res = await fetch(`${API_BASE_URL}${path}`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(body),
  });

  if (!res.ok) {
    const text = await res.text();
    throw new Error(text || `HTTP ${res.status}`);
  }

  return res.json() as Promise<T>;
}
