import { createServerFn } from "@tanstack/react-start"
import { getRequestHeader, setResponseHeader, getResponseHeaders } from "@tanstack/react-start/server"
import { getAuthConfig, loginURL, callGatewayLogout } from "./auth-gateway.server"
import { getCurrentUser } from "./auth.server"
import { redirect } from "@tanstack/react-router"

export const getRouteSession = createServerFn({ method: "POST" })
  .validator((returnTo: string) => {
    if (typeof returnTo !== "string") {
      throw new Error("Authentication return path must be a string")
    }
    return returnTo
  })
  .handler(async ({ data }) => {
    const loginHref = loginURL(data, getAuthConfig())
    const user = await getCurrentUser()
    return { user, loginHref }
  })

export const handleLogout = createServerFn({ method: "POST" })
  .handler(async () => {
    const cookieHeader = getRequestHeader("cookie") ?? ""
    const { cookies } = await callGatewayLogout(cookieHeader, getAuthConfig())
    
    setResponseHeader("Cache-Control", "private, no-store")
    setResponseHeader("Vary", "Cookie")
    
    // Set logout cookies on response
    for (const cookie of cookies) {
      getResponseHeaders().append("Set-Cookie", cookie)
    }
    
    throw redirect({ to: "/" })
  })