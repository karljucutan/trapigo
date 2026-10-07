import { createMiddleware, createServerFn } from "@tanstack/react-start"
import { setResponseStatus } from "@tanstack/react-start/server"
import { getAuthConfig, loginURL } from "./auth-gateway.server"
import { getCurrentUser } from "./auth.server"

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