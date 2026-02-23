import { useState, useMemo } from "react";
import { useQuery } from "@tanstack/react-query";
import { api } from "@/lib/api";
import { Select } from "@/components/ui/select";
import { Card, CardHeader, CardTitle, CardContent } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { LoadingSpinner } from "@/components/ui/loading-spinner";
import type { CoverageResponse, CoverageSlot } from "@/types/api";

const CELL_W = 28;
const CELL_H = 20;
const LABEL_W = 60;
const HEADER_H = 24;
const HOURS = Array.from({ length: 24 }, (_, i) => i);

function formatHour(h: number): string {
  return String(h).padStart(2, "0");
}

function formatDate(iso: string, tz: string): string {
  try {
    return new Date(iso).toLocaleDateString("en-US", {
      timeZone: tz,
      weekday: "short",
      month: "short",
      day: "numeric",
    });
  } catch {
    return new Date(iso).toLocaleDateString("en-US", { weekday: "short", month: "short", day: "numeric" });
  }
}

interface DayRow {
  label: string;
  slots: CoverageSlot[];
}

export function CoverageHeatmap() {
  const [displayTz, setDisplayTz] = useState("UTC");

  const { data, isLoading } = useQuery({
    queryKey: ["roster-coverage"],
    queryFn: () => api.get<CoverageResponse>("/rosters/coverage"),
  });

  const tzOptions = useMemo(() => {
    const tzs = new Set(["UTC"]);
    if (data) {
      for (const r of data.rosters) {
        tzs.add(r.timezone);
      }
    }
    return Array.from(tzs);
  }, [data]);

  const days = useMemo<DayRow[]>(() => {
    if (!data?.slots?.length) return [];

    // Group slots by day in the display timezone.
    const dayMap = new Map<string, CoverageSlot[]>();
    for (const slot of data.slots) {
      const d = new Date(slot.time);
      let dayKey: string;
      try {
        dayKey = d.toLocaleDateString("en-CA", { timeZone: displayTz }); // YYYY-MM-DD
      } catch {
        dayKey = d.toISOString().slice(0, 10);
      }
      if (!dayMap.has(dayKey)) dayMap.set(dayKey, []);
      dayMap.get(dayKey)!.push(slot);
    }

    return Array.from(dayMap.entries()).map(([key, slots]) => ({
      label: formatDate(slots[0].time, displayTz),
      slots,
    }));
  }, [data, displayTz]);

  const [tooltip, setTooltip] = useState<{
    x: number;
    y: number;
    slot: CoverageSlot;
  } | null>(null);

  if (isLoading) return <LoadingSpinner size="sm" label="Loading coverage..." />;
  if (!data || data.rosters.length === 0) return null;

  const svgWidth = LABEL_W + HOURS.length * CELL_W + 4;
  const svgHeight = HEADER_H + days.length * CELL_H + 4;

  return (
    <Card>
      <CardHeader>
        <div className="flex items-center justify-between">
          <CardTitle>Coverage — Next 14 Days</CardTitle>
          <div className="flex items-center gap-2">
            <span className="text-xs text-muted-foreground">Timezone:</span>
            <Select
              value={displayTz}
              onChange={(e) => setDisplayTz(e.target.value)}
              className="w-48 text-xs"
            >
              {tzOptions.map((tz) => (
                <option key={tz} value={tz}>{tz}</option>
              ))}
            </Select>
          </div>
        </div>
      </CardHeader>
      <CardContent>
        <div className="overflow-x-auto relative">
          <svg
            width={svgWidth}
            height={svgHeight}
            className="font-mono text-[10px]"
            onMouseLeave={() => setTooltip(null)}
          >
            {/* Hour headers */}
            {HOURS.map((h) => (
              <text
                key={h}
                x={LABEL_W + h * CELL_W + CELL_W / 2}
                y={HEADER_H - 6}
                textAnchor="middle"
                className="fill-muted-foreground"
                fontSize={9}
              >
                {h % 2 === 0 ? formatHour(h) : ""}
              </text>
            ))}

            {/* Day rows */}
            {days.map((day, dayIdx) => (
              <g key={dayIdx} transform={`translate(0, ${HEADER_H + dayIdx * CELL_H})`}>
                <text
                  x={LABEL_W - 4}
                  y={CELL_H / 2 + 3}
                  textAnchor="end"
                  className="fill-muted-foreground"
                  fontSize={10}
                >
                  {day.label}
                </text>

                {HOURS.map((h) => {
                  const slot = day.slots[h];
                  if (!slot) {
                    return (
                      <rect
                        key={h}
                        x={LABEL_W + h * CELL_W}
                        y={0}
                        width={CELL_W - 1}
                        height={CELL_H - 1}
                        rx={2}
                        className="fill-muted/30"
                      />
                    );
                  }

                  const isGap = slot.gap;
                  const hasMultiple = slot.coverage.length > 1;
                  const isUnassigned = !isGap && slot.coverage.some((c) => c.source === "unassigned");

                  let fillClass = "fill-amber-600/60"; // single roster covered
                  if (isGap) fillClass = "fill-red-900/50 stroke-red-500/40";
                  else if (isUnassigned) fillClass = "fill-amber-500/30 stroke-amber-500/40";
                  else if (hasMultiple) fillClass = "fill-amber-600";

                  return (
                    <rect
                      key={h}
                      x={LABEL_W + h * CELL_W}
                      y={0}
                      width={CELL_W - 1}
                      height={CELL_H - 1}
                      rx={2}
                      className={fillClass}
                      strokeWidth={isGap || isUnassigned ? 1 : 0}
                      onMouseEnter={(e) => {
                        const rect = e.currentTarget.getBoundingClientRect();
                        setTooltip({
                          x: rect.left + rect.width / 2,
                          y: rect.top,
                          slot,
                        });
                      }}
                      onMouseLeave={() => setTooltip(null)}
                      style={{ cursor: "pointer" }}
                    />
                  );
                })}
              </g>
            ))}
          </svg>

          {/* Tooltip */}
          {tooltip && (
            <div
              className="fixed z-50 bg-popover border border-border rounded-md shadow-lg p-2 text-xs max-w-64 pointer-events-none"
              style={{
                left: tooltip.x,
                top: tooltip.y - 8,
                transform: "translate(-50%, -100%)",
              }}
            >
              <div className="font-medium mb-1">
                {new Date(tooltip.slot.time).toLocaleString("en-US", {
                  timeZone: displayTz,
                  weekday: "short",
                  month: "short",
                  day: "numeric",
                  hour: "2-digit",
                  minute: "2-digit",
                  hour12: false,
                })}
              </div>
              {tooltip.slot.gap ? (
                <div className="text-red-400">No roster active</div>
              ) : (
                tooltip.slot.coverage.map((c, i) => (
                  <div key={i} className="mt-1">
                    <div className="text-muted-foreground">{c.roster_name}</div>
                    <div>Primary: {c.primary || "Unassigned"}</div>
                    {c.secondary && <div className="text-muted-foreground">Secondary: {c.secondary}</div>}
                    <div className="text-muted-foreground">Source: {c.source}</div>
                  </div>
                ))
              )}
            </div>
          )}
        </div>

        {/* Legend */}
        <div className="flex items-center gap-4 mt-3 text-xs text-muted-foreground">
          <div className="flex items-center gap-1">
            <span className="inline-block w-3 h-3 rounded-sm bg-amber-600/60" />
            Covered
          </div>
          <div className="flex items-center gap-1">
            <span className="inline-block w-3 h-3 rounded-sm bg-amber-600" />
            Overlap
          </div>
          <div className="flex items-center gap-1">
            <span className="inline-block w-3 h-3 rounded-sm bg-red-900/50 border border-red-500/40" />
            Gap
          </div>
          <div className="flex items-center gap-1">
            <span className="inline-block w-3 h-3 rounded-sm bg-amber-500/30 border border-amber-500/40" />
            Unassigned
          </div>
        </div>

        {/* Gap summary */}
        {data.gap_summary.total_gap_hours > 0 && (
          <div className="mt-3 flex items-start gap-2 rounded-md bg-destructive/10 border border-destructive/20 p-3">
            <Badge variant="destructive" className="text-xs shrink-0">Gap</Badge>
            <div className="text-xs">
              <p>
                {data.gap_summary.total_gap_hours.toFixed(1)}h total uncovered in the next 14 days
                ({data.gap_summary.gaps.length} gap{data.gap_summary.gaps.length !== 1 ? "s" : ""})
              </p>
              <p className="text-muted-foreground mt-1">
                Consider extending active hours or adding an overnight roster.
              </p>
            </div>
          </div>
        )}
      </CardContent>
    </Card>
  );
}
