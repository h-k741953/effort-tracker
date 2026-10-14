import { useState } from "react";
import { ThemeToggle } from "web";

export const Light = () => {
  const [theme, setTheme] = useState<"light" | "dark">("light");
  return <ThemeToggle theme={theme} onChange={setTheme} />;
};

export const Dark = () => {
  const [theme, setTheme] = useState<"light" | "dark">("dark");
  return <ThemeToggle theme={theme} onChange={setTheme} />;
};
