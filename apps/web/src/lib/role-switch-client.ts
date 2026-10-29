// docs/specs/web-screens-implementation-design.md L-4。
// POST /api/auth/role を本文なしで送る（ロールを申告しない）。

export type RoleSwitchResult = "switched" | "signed-out" | "failed";

export async function requestRoleSwitch(
  fetchImpl: typeof fetch = fetch,
): Promise<RoleSwitchResult> {
  try {
    const response = await fetchImpl("/api/auth/role", { method: "POST" });
    if (response.status === 204) return "switched";
    if (response.status === 401) return "signed-out";
    return "failed";
  } catch {
    return "failed";
  }
}
