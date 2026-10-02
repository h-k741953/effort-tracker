import { useState } from "react";
import { RoleSwitcher } from "web";

export const Engineer = () => {
  const [role, setRole] = useState<"Engineer" | "Approver">("Engineer");
  return <RoleSwitcher role={role} onChange={setRole} />;
};

export const Approver = () => {
  const [role, setRole] = useState<"Engineer" | "Approver">("Approver");
  return <RoleSwitcher role={role} onChange={setRole} />;
};
