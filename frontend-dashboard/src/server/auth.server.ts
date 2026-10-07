import {
  getRequestHeader,
  getResponseHeaders,
  setResponseHeader,
} from "@tanstack/react-start/server"
import { fetchGatewaySession, getAuthConfig } from "./auth-gateway.server"

export async function getCurrentUser() {
  setResponseHeader("Cache-Control", "private, no-store")
  setResponseHeader("Vary", "Cookie")
  const session = await fetchGatewaySession(
    getRequestHeader("cookie") ?? "",
    getAuthConfig()
  )
  // Preserve gateway refresh/clear cookies on both SSR and server-function responses.
  for (const cookie of session.cookies) {
    getResponseHeaders().append("Set-Cookie", cookie)
  }
  return session.user
}
