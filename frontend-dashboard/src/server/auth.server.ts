import {
  getRequestHeader,
  getResponseHeaders,
  setResponseHeader,
} from "@tanstack/react-start/server";
import { fetchGatewaySession, getAuthConfig } from "./auth-gateway.server";

export function setPrivateNoStoreHeaders() {
  setResponseHeader("Cache-Control", "private, no-store");
  const vary = new Set(
    (getResponseHeaders().get("Vary") ?? "")
      .split(",")
      .map((value) => value.trim())
      .filter(Boolean),
  );
  vary.add("Cookie");
  setResponseHeader("Vary", [...vary].join(", "));
}

export function forwardAuthCookies(cookies: string[]) {
  if (cookies.length === 0) return;
  const existingCookies = getResponseHeaders().getSetCookie();
  setResponseHeader("Set-Cookie", [...existingCookies, ...cookies]);
}

export async function getCurrentUser() {
  setPrivateNoStoreHeaders();
  const cookieHeader = getRequestHeader("cookie") ?? "";
  const session = await fetchGatewaySession(cookieHeader, getAuthConfig());
  forwardAuthCookies(session.cookies);
  return session.user;
}
