import type { Metadata } from "next";
import "./globals.css";

export const metadata: Metadata = {
  title: "APA — تخصیص هوشمند کار",
  description:
    "درخواست مدیر را به برنامه‌ای تأییدشده، تخصیص‌یافته و بررسی‌شده تبدیل کنید؛ سیستمی که از سازمان شما یاد می‌گیرد.",
};

export default function RootLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  return (
    <html lang="fa" dir="rtl">
      <body>{children}</body>
    </html>
  );
}
