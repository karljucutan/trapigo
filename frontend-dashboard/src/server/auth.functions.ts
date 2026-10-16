import { createServerFn } from "@tanstack/react-start";
import { getCurrentUser } from "./auth.server";
import {
  getAuthConfig,
  loginURL,
  logoutActionURL,
} from "./auth-gateway.server";

export const getRouteSession = createServerFn({ method: "POST" })
  .validator((returnTo: string) => {
    if (typeof returnTo !== "string") {
      throw new Error("Authentication return path must be a string");
    }
    return returnTo;
  })
  .handler(async ({ data }) => {
    const loginHref = loginURL(data, getAuthConfig());
    const user = await getCurrentUser();
    const logoutAction = logoutActionURL(getAuthConfig());
    return { user, loginHref, logoutAction };
  });
