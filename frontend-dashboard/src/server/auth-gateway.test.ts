import assert from "node:assert/strict"
import { test } from "node:test"
import {
  fetchGatewaySession,
  getAuthConfig,
  loginURL,
} from "./auth-gateway.server.ts"

const config = {
  appOrigin: "http://localhost:3000",
  gatewayOrigin: "http://localhost",
  loginOrigin: "http://localhost",
}

test("production configuration requires explicit HTTPS origins", (t) => {
  const names = [
    "NODE_ENV",
    "APP_ORIGIN",
    "AUTH_PUBLIC_ORIGIN",
    "AUTH_GATEWAY_ORIGIN",
  ]
  const original = names.map((name) => [name, process.env[name]] as const)
  t.after(() => {
    for (const [name, value] of original) {
      if (value === undefined) delete process.env[name]
      else process.env[name] = value
    }
  })
  process.env.NODE_ENV = "production"
  delete process.env.APP_ORIGIN
  delete process.env.AUTH_PUBLIC_ORIGIN
  delete process.env.AUTH_GATEWAY_ORIGIN
  assert.throws(getAuthConfig, /required in production/)
  process.env.APP_ORIGIN = "http://app.example.com"
  process.env.AUTH_PUBLIC_ORIGIN = "https://app.example.com"
  assert.throws(getAuthConfig, /HTTPS/)
  process.env.APP_ORIGIN = "https://app.example.com"
  assert.deepEqual(getAuthConfig(), {
    appOrigin: "https://app.example.com",
    gatewayOrigin: "https://app.example.com",
    loginOrigin: "https://app.example.com",
  })
  process.env.AUTH_PUBLIC_ORIGIN = "https://app.example.com/login"
  assert.throws(getAuthConfig, /without a path/)
})

test("login URL retains the dashboard URL, query and fragment", () => {
  const url = new URL(loginURL("/dashboard?tab=logs#latest", config))
  assert.equal(url.origin + url.pathname, "http://localhost/web/auth/login")
  assert.equal(
    url.searchParams.get("return_to"),
    "http://localhost:3000/dashboard?tab=logs#latest"
  )
})

test("login URL rejects external and malformed return paths", () => {
  for (const path of [
    "//evil.example/dashboard",
    "/\\evil.example",
    "https://evil.example",
    "dashboard",
    "/dashboard\n",
    `/${"x".repeat(4096)}`,
  ]) {
    assert.throws(() => loginURL(path, config))
  }
})

test("session validates at the gateway and forwards only auth cookies", async () => {
  const request: typeof fetch = async (url, options) => {
    assert.equal(String(url), "http://localhost/web/auth/me")
    assert.equal(
      new Headers(options?.headers).get("cookie"),
      "web_access_token=access; web_refresh_token=refresh"
    )
    assert.equal(options?.cache, "no-store")
    assert.equal(options.redirect, "error")
    assert.ok(options.signal)
    return Response.json(
      { subject: "user-1", username: "karl", email: "private@example.com" },
      {
        headers: [
          ["Set-Cookie", "web_access_token=rotated; Path=/; HttpOnly"],
          ["Set-Cookie", "unrelated=value; Path=/"],
        ],
      }
    )
  }
  const session = await fetchGatewaySession(
    "other=secret; web_access_token=access; web_refresh_token=refresh",
    config,
    request
  )
  assert.deepEqual(session.user, { subject: "user-1", username: "karl" })
  assert.deepEqual(session.cookies, [
    "web_access_token=rotated; Path=/; HttpOnly",
  ])
})

test("only 401 is treated as unauthenticated and retains cookie clearing", async () => {
  const request: typeof fetch = async () =>
    new Response(null, {
      status: 401,
      headers: { "Set-Cookie": "web_access_token=; Max-Age=0; Path=/" },
    })
  assert.deepEqual(await fetchGatewaySession("", config, request), {
    user: null,
    cookies: ["web_access_token=; Max-Age=0; Path=/"],
  })
  for (const status of [403, 500, 503]) {
    const failingRequest: typeof fetch = async () =>
      new Response(null, { status })
    await assert.rejects(
      fetchGatewaySession("", config, failingRequest),
      new RegExp(String(status))
    )
  }
})

test("invalid gateway payloads and network failures surface as errors", async () => {
  for (const body of [null, {}, { subject: "", username: "name" }]) {
    const request: typeof fetch = async () => Response.json(body)
    await assert.rejects(
      fetchGatewaySession("", config, request),
      /invalid user/
    )
  }
  const request: typeof fetch = async () => {
    throw new Error("gateway unavailable")
  }
  await assert.rejects(
    fetchGatewaySession("", config, request),
    /gateway unavailable/
  )
})
