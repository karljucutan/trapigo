export type CurrentUser = {
  subject: string
  username: string
}

export type AuthConfig = {
  appOrigin: string
  gatewayOrigin: string
  loginOrigin: string
}

function configuredOrigin(value: string, name: string): string {
  const url = new URL(value)
  if (
    !["http:", "https:"].includes(url.protocol) ||
    url.username ||
    url.password ||
    url.pathname !== "/" ||
    url.search ||
    url.hash
  ) {
    throw new Error(`${name} must be an HTTP(S) origin without a path`)
  }
  if (process.env.NODE_ENV === "production" && url.protocol !== "https:") {
    throw new Error(`${name} must use HTTPS in production`)
  }
  return url.origin
}

export function getAuthConfig(): AuthConfig {
  const production = process.env.NODE_ENV === "production"
  const appOrigin = process.env.APP_ORIGIN
  const loginOrigin = process.env.AUTH_PUBLIC_ORIGIN
  if (production && (!appOrigin || !loginOrigin)) {
    throw new Error(
      "APP_ORIGIN and AUTH_PUBLIC_ORIGIN are required in production"
    )
  }
  return {
    appOrigin: configuredOrigin(
      appOrigin ?? "http://localhost:3000",
      "APP_ORIGIN"
    ),
    loginOrigin: configuredOrigin(
      loginOrigin ?? "http://localhost",
      "AUTH_PUBLIC_ORIGIN"
    ),
    gatewayOrigin: configuredOrigin(
      process.env.AUTH_GATEWAY_ORIGIN ?? loginOrigin ?? "http://localhost",
      "AUTH_GATEWAY_ORIGIN"
    ),
  }
}

export function loginURL(returnTo: string, config: AuthConfig): string {
  if (
    !returnTo.startsWith("/") ||
    returnTo.startsWith("//") ||
    returnTo.includes("\\") ||
    [...returnTo].some(
      (char) => char.charCodeAt(0) < 32 || char.charCodeAt(0) === 127
    ) ||
    returnTo.length > 4096
  ) {
    throw new Error("Invalid authentication return path")
  }
  const target = new URL(returnTo, config.appOrigin)
  if (target.origin !== config.appOrigin) {
    throw new Error("Authentication return URL must be on the frontend origin")
  }
  const login = new URL("/web/auth/login", config.loginOrigin)
  login.searchParams.set("return_to", target.href)
  return login.href
}

export async function fetchGatewaySession(
  cookieHeader: string,
  config: AuthConfig,
  request: typeof fetch = fetch
): Promise<{ user: CurrentUser | null; cookies: string[] }> {
  const cookies = cookieHeader
    .split(";")
    .map((cookie) => cookie.trim())
    .filter((cookie) => /^web_(access|refresh)_token=/.test(cookie))
    .join("; ")
  const response = await request(
    new URL("/web/auth/me", config.gatewayOrigin),
    {
      headers: cookies ? { Cookie: cookies } : {},
      cache: "no-store",
      redirect: "error",
      signal: AbortSignal.timeout(5000),
    }
  )
  const rotatedCookies = response.headers
    .getSetCookie()
    .filter((cookie) => /^web_(access|refresh)_token=/.test(cookie))

  if (response.status === 401) {
    return { user: null, cookies: rotatedCookies }
  }
  if (!response.ok) {
    throw new Error(`Authentication gateway returned status ${response.status}`)
  }
  const body: unknown = await response.json()
  if (
    typeof body !== "object" ||
    body === null ||
    !("subject" in body) ||
    typeof body.subject !== "string" ||
    !body.subject ||
    !("username" in body) ||
    typeof body.username !== "string"
  ) {
    throw new Error("Authentication gateway returned an invalid user")
  }
  return {
    user: { subject: body.subject, username: body.username },
    cookies: rotatedCookies,
  }
}

export async function callGatewayLogout(
  cookieHeader: string,
  config: AuthConfig,
  request: typeof fetch = fetch
): Promise<{ cookies: string[] }> {
  const cookies = cookieHeader
    .split(";")
    .map((cookie) => cookie.trim())
    .filter((cookie) => /^web_(access|refresh)_token=/.test(cookie))
    .join("; ")
  const response = await request(
    new URL("/web/auth/logout", config.gatewayOrigin),
    {
      method: "POST",
      headers: cookies ? { Cookie: cookies } : {},
      cache: "no-store",
      redirect: "error",
      signal: AbortSignal.timeout(5000),
    }
  )
  const rotatedCookies = response.headers
    .getSetCookie()
    .filter((cookie) => /^web_(access|refresh)_token=/.test(cookie))

  if (!response.ok) {
    throw new Error(`Logout failed with status ${response.status}`)
  }

  return { cookies: rotatedCookies }
}
