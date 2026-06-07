import { useState } from 'react';
import { Sun, Moon, Wifi, WifiOff, Zap, CalendarDays, Plug, BatteryCharging, Thermometer, Activity, Home, SlidersHorizontal } from 'lucide-react';
import { useSSE } from '@/hooks/useSSE';
import { useTheme } from '@/contexts/ThemeContext';
import { formatPower, formatEnergy } from '@/types/status';
import { DetailView } from '@/components/DetailView';
import type { ReactNode } from 'react';

interface StatCardProps {
  icon: ReactNode;
  label: string;
  value: string;
  sub?: string;
}

function StatCard({ icon, label, value, sub }: StatCardProps) {
  return (
    <div className="p-4 bg-card rounded-lg border border-border">
      <div className="flex items-center gap-2 text-muted-foreground text-sm mb-1">
        {icon}
        <span>{label}</span>
      </div>
      <div className="text-xl font-semibold text-foreground tabular-nums">{value}</div>
      {sub && <div className="text-xs text-muted-foreground mt-0.5">{sub}</div>}
    </div>
  );
}

export function App() {
  const { status, isConnected, error, reconnect } = useSSE();
  const { theme, toggleTheme } = useTheme();
  const [showDetails, setShowDetails] = useState(false);

  if (showDetails) {
    return <DetailView status={status} onBack={() => setShowDetails(false)} />;
  }

  const online = (status?.connected ?? false) && isConnected;
  const pv = status?.pvPower ?? 0;

  // Grid power: positive = importing from grid, negative = exporting.
  const grid = status?.gridPower;
  const gridValue = grid === undefined ? '—' : formatPower(Math.abs(grid));
  const gridSub = grid === undefined ? 'no meter' : grid >= 0 ? 'importing' : 'exporting';

  const battery = status?.battery;

  return (
    <div className="min-h-screen bg-background p-4 md:p-8">
      <div className="max-w-2xl mx-auto">
        {/* Header */}
        <div className="flex items-center justify-between mb-8">
          <div className="flex items-center gap-3">
            <Sun className="h-8 w-8 text-primary" />
            <h1 className="text-2xl font-bold text-foreground">Huawei Solar</h1>
          </div>
          <div className="flex items-center gap-1">
            <div className="p-2" title={online ? 'Connected' : 'Disconnected'}>
              {online ? (
                <Wifi className="h-5 w-5 text-green-500" />
              ) : (
                <WifiOff className="h-5 w-5 text-red-500 cursor-pointer" onClick={reconnect} />
              )}
            </div>
            <button
              onClick={() => setShowDetails(true)}
              className="p-2 rounded-lg hover:bg-accent transition-colors"
              aria-label="Details"
              title="Details"
            >
              <SlidersHorizontal className="h-5 w-5 text-foreground" />
            </button>
            <button
              onClick={toggleTheme}
              className="p-2 rounded-lg hover:bg-accent transition-colors"
              aria-label="Toggle theme"
            >
              {theme === 'dark' ? (
                <Sun className="h-5 w-5 text-foreground" />
              ) : (
                <Moon className="h-5 w-5 text-foreground" />
              )}
            </button>
          </div>
        </div>

        {/* Error message */}
        {error && (
          <div className="mb-4 p-3 bg-red-500/10 border border-red-500/20 rounded-lg text-red-500 text-sm">
            {error}
            <button onClick={reconnect} className="ml-2 underline hover:no-underline">
              Retry
            </button>
          </div>
        )}

        {/* Device info */}
        {status && (
          <div className="mb-4 flex items-center justify-between text-sm text-muted-foreground">
            <span>
              {status.model || 'Huawei SUN2000'}
              {status.serial && ` (${status.serial})`}
            </span>
            <span className="uppercase tracking-wide text-xs">{status.source}</span>
          </div>
        )}

        {/* Solar production hero */}
        <div className="mb-6 p-6 bg-card rounded-lg border border-border text-center">
          <div className="flex items-center justify-center gap-2 text-muted-foreground text-sm mb-2">
            <Sun className="h-4 w-4 text-primary" />
            <span>Solar Production</span>
          </div>
          <div className="text-5xl font-bold text-primary tabular-nums">
            {formatPower(pv)}
          </div>
          {status?.state && (
            <div className="mt-2 inline-flex items-center gap-1.5 text-sm text-muted-foreground">
              <Activity className="h-4 w-4" />
              {status.state}
            </div>
          )}
        </div>

        {/* Stats grid */}
        <div className="grid grid-cols-2 gap-3">
          {status?.consumption !== undefined && (
            <StatCard
              icon={<Home className="h-4 w-4" />}
              label="House Consumption"
              value={formatPower(status.consumption)}
            />
          )}
          <StatCard
            icon={<Plug className="h-4 w-4" />}
            label="Grid"
            value={gridValue}
            sub={gridSub}
          />
          {battery && (
            <StatCard
              icon={<BatteryCharging className="h-4 w-4" />}
              label={`Battery · ${battery.soc.toFixed(0)}%`}
              value={formatPower(Math.abs(battery.power))}
              sub={battery.power >= 0 ? 'charging' : 'discharging'}
            />
          )}
          <StatCard
            icon={<Zap className="h-4 w-4" />}
            label="Inverter Output"
            value={formatPower(status?.activePower ?? 0)}
            sub="AC active power"
          />
          <StatCard
            icon={<CalendarDays className="h-4 w-4" />}
            label="Today"
            value={formatEnergy(status?.dailyYield ?? 0)}
          />
          <StatCard
            icon={<Sun className="h-4 w-4" />}
            label="Total Yield"
            value={formatEnergy(status?.totalYield ?? 0)}
          />
          {status?.temperature !== undefined && (
            <StatCard
              icon={<Thermometer className="h-4 w-4" />}
              label="Inverter Temp"
              value={`${status.temperature.toFixed(1)} °C`}
            />
          )}
        </div>

        {/* Footer */}
        <div className="mt-8 text-center text-xs text-muted-foreground">
          mqtt-huawei
          {status?.updatedAt && (
            <span> · updated {new Date(status.updatedAt).toLocaleTimeString()}</span>
          )}
        </div>
      </div>
    </div>
  );
}
