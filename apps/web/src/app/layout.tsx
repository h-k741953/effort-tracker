import type { Metadata } from "next";
import "./globals.css";

export const metadata: Metadata = {
  title: "effort-tracker",
  description: "SES/受託向けの勤怠・工数管理 SaaS",
};

export default function RootLayout({
  children,
}: Readonly<{ children: React.ReactNode }>) {
  return (
    <html lang="en" className="h-full antialiased">
      <body className="min-h-full flex flex-col">{children}</body>
    </html>
  );
}
