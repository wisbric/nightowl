import { useState, useRef, useEffect } from "react";
import { useQuery } from "@tanstack/react-query";
import { api } from "@/lib/api";
import { Input } from "@/components/ui/input";
import type { UsersResponse, UserDetail } from "@/types/api";

interface UserSearchSelectProps {
  onSelect: (user: UserDetail) => void;
  placeholder?: string;
  excludeUserIds?: string[];
  /** When set, users matching this timezone sort to the top. */
  rosterTimezone?: string;
}

export function UserSearchSelect({ onSelect, placeholder = "Search users...", excludeUserIds = [], rosterTimezone }: UserSearchSelectProps) {
  const [search, setSearch] = useState("");
  const [open, setOpen] = useState(false);
  const containerRef = useRef<HTMLDivElement>(null);

  const { data } = useQuery({
    queryKey: ["users"],
    queryFn: () => api.get<UsersResponse>("/users"),
  });

  // Filter by search text, exclude already-added users, then sort matching timezone first.
  const users = (data?.users ?? [])
    .filter(
      (u) =>
        !excludeUserIds.includes(u.id) &&
        (search.length === 0 ||
          u.display_name.toLowerCase().includes(search.toLowerCase()) ||
          u.email.toLowerCase().includes(search.toLowerCase()))
    )
    .sort((a, b) => {
      if (!rosterTimezone) return 0;
      const aMatch = a.timezone === rosterTimezone ? 0 : 1;
      const bMatch = b.timezone === rosterTimezone ? 0 : 1;
      return aMatch - bMatch;
    });

  useEffect(() => {
    function handleClickOutside(e: MouseEvent) {
      if (containerRef.current && !containerRef.current.contains(e.target as Node)) {
        setOpen(false);
      }
    }
    document.addEventListener("mousedown", handleClickOutside);
    return () => document.removeEventListener("mousedown", handleClickOutside);
  }, []);

  return (
    <div ref={containerRef} className="relative">
      <Input
        placeholder={placeholder}
        value={search}
        onChange={(e) => {
          setSearch(e.target.value);
          setOpen(true);
        }}
        onFocus={() => setOpen(true)}
      />
      {open && users.length > 0 && (
        <ul className="absolute z-50 mt-1 max-h-48 w-full overflow-auto rounded-md border bg-popover p-1 shadow-md">
          {users.slice(0, 10).map((user) => (
            <li key={user.id}>
              <button
                type="button"
                className="flex w-full items-center gap-2 rounded-sm px-2 py-1.5 text-sm hover:bg-accent hover:text-accent-foreground"
                onClick={() => {
                  onSelect(user);
                  setSearch("");
                  setOpen(false);
                }}
              >
                <span className="font-medium">{user.display_name}</span>
                <span className="text-xs text-muted-foreground">{user.email}</span>
                <span className={`ml-auto text-xs ${rosterTimezone && user.timezone === rosterTimezone ? "text-green-500" : "text-muted-foreground"}`}>
                  {user.timezone}
                </span>
              </button>
            </li>
          ))}
        </ul>
      )}
      {open && users.length === 0 && (
        <div className="absolute z-50 mt-1 w-full rounded-md border bg-popover p-2 shadow-md">
          <p className="text-sm text-muted-foreground">
            {search.length > 0 ? "No matching users found" : "No available users"}
          </p>
        </div>
      )}
    </div>
  );
}
