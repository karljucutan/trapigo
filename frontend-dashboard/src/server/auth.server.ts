import {
  getRequestHeader,
  setResponseHeader,
} from "@tanstack/react-start/server";
import { fetchGatewaySession, getAuthConfig } from "./auth-gateway.server";

export async function getCurrentUser() {
  setResponseHeader("Cache-Control", "private, no-store");
  setResponseHeader("Vary", "Cookie");
  const session = await fetchGatewaySession(
    getRequestHeader("cookie") ?? "",
    getAuthConfig(),
  );
  if (session.cookies.length > 0) {
    setResponseHeader("Set-Cookie", session.cookies);
  }
  return session.user;
}
