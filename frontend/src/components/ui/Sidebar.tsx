"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";

const nav = [
  { label: "Overview",  href: "/dashboard",          icon: "▦" },
  { label: "Payments",  href: "/dashboard/payments",  icon: "↕" },
  { label: "Wallet",    href: "/dashboard/wallet",    icon: "◎" },
  { label: "Webhooks",  href: "/dashboard/webhooks",  icon: "⇌" },
];

export function Sidebar() {
  const pathname = usePathname();

  return (
    <aside className="flex h-screen w-60 flex-col border-r border-gray-100 bg-white px-4 py-6">
      {/* Brand */}
      <div className="mb-8 flex items-center gap-2 px-2">
        <span className="text-xl font-black tracking-tight text-indigo-600">
          Pay
        </span>
        <span className="text-xl font-black tracking-tight text-gray-900">
          Flow
        </span>
      </div>

      {/* Nav links */}
      <nav className="flex flex-1 flex-col gap-1">
        {nav.map(({ label, href, icon }) => {
          const active =
            href === "/dashboard"
              ? pathname === "/dashboard"
              : pathname.startsWith(href);

          return (
            <Link
              key={href}
              href={href}
              className={`flex items-center gap-3 rounded-xl px-3 py-2.5 text-sm font-medium transition-colors ${
                active
                  ? "bg-indigo-50 text-indigo-700"
                  : "text-gray-600 hover:bg-gray-50 hover:text-gray-900"
              }`}
            >
              <span className="text-base">{icon}</span>
              {label}
            </Link>
          );
        })}
      </nav>

      {/* Footer */}
      <div className="border-t border-gray-100 pt-4">
        <button
          onClick={() => {
            localStorage.removeItem("access_token");
            window.location.href = "/";
          }}
          className="flex w-full items-center gap-3 rounded-xl px-3 py-2.5 text-sm font-medium text-gray-500 transition-colors hover:bg-red-50 hover:text-red-600"
        >
          <span>⎋</span>
          Sign out
        </button>
      </div>
    </aside>
  );
}
