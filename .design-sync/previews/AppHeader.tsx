import { useState } from "react";
import { AppHeader, RoleSwitcher } from "web";

export const Plain = () => <AppHeader />;

export const WithRoleSwitcher = () => {
  const [role, setRole] = useState<"Engineer" | "Approver">("Engineer");
  return <AppHeader right={<RoleSwitcher role={role} onChange={setRole} />} />;
};
