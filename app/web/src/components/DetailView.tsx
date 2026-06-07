import { ArrowLeft, Sun, Gauge, Plug, BatteryCharging, Info } from 'lucide-react';
import type { ReactNode } from 'react';
import { InverterStatus, formatPower, formatEnergy } from '@/types/status';

interface RowProps {
  label: string;
  value: ReactNode;
}

function Row({ label, value }: RowProps) {
  return (
    <div className="flex items-center justify-between py-2 border-b border-border last:border-0">
      <span className="text-sm text-muted-foreground">{label}</span>
      <span className="text-sm font-medium text-foreground tabular-nums">{value}</span>
    </div>
  );
}

interface SectionProps {
  icon: ReactNode;
  title: string;
  children: ReactNode;
}

function Section({ icon, title, children }: SectionProps) {
  return (
    <div className="mb-4 p-4 bg-card rounded-lg border border-border">
      <div className="flex items-center gap-2 mb-2 text-foreground font-medium">
        {icon}
        <h2>{title}</h2>
      </div>
      {children}
    </div>
  );
}

const v = (n: number) => `${n.toFixed(1)} V`;
const a = (n: number) => `${n.toFixed(2)} A`;
const hz = (n: number) => `${n.toFixed(2)} Hz`;

interface DetailViewProps {
  status: InverterStatus | null;
  onBack: () => void;
}

export function DetailView({ status, onBack }: DetailViewProps) {
  const d = status?.details;

  return (
    <div className="min-h-screen bg-background p-4 md:p-8">
      <div className="max-w-2xl mx-auto">
        {/* Header */}
        <div className="flex items-center gap-3 mb-6">
          <button
            onClick={onBack}
            className="p-2 rounded-lg hover:bg-accent transition-colors"
            aria-label="Back"
          >
            <ArrowLeft className="h-5 w-5 text-foreground" />
          </button>
          <h1 className="text-2xl font-bold text-foreground">Details</h1>
        </div>

        {/* Device */}
        <Section icon={<Info className="h-4 w-4" />} title="Device">
          <Row label="Model" value={status?.model || '—'} />
          <Row label="Serial number" value={status?.serial || '—'} />
          <Row label="State" value={status?.state || '—'} />
          <Row label="Source" value={status?.source || '—'} />
          {status?.temperature !== undefined && (
            <Row label="Inverter temperature" value={`${status.temperature.toFixed(1)} °C`} />
          )}
          {status?.updatedAt && (
            <Row label="Updated" value={new Date(status.updatedAt).toLocaleTimeString()} />
          )}
        </Section>

        {/* PV strings */}
        {d?.pv && d.pv.length > 0 && (
          <Section icon={<Sun className="h-4 w-4" />} title="PV Strings (DC)">
            {d.pv.map((s, i) => (
              <Row
                key={i}
                label={`String ${i + 1}`}
                value={`${v(s.voltage)} · ${a(s.current)} · ${formatPower(s.power)}`}
              />
            ))}
          </Section>
        )}

        {/* Inverter AC */}
        {d?.ac && (
          <Section icon={<Gauge className="h-4 w-4" />} title="Inverter AC">
            <Row label="Phase A" value={`${v(d.ac.phaseAVoltage)} · ${a(d.ac.phaseACurrent)}`} />
            <Row label="Phase B" value={`${v(d.ac.phaseBVoltage)} · ${a(d.ac.phaseBCurrent)}`} />
            <Row label="Phase C" value={`${v(d.ac.phaseCVoltage)} · ${a(d.ac.phaseCCurrent)}`} />
            <Row label="Frequency" value={hz(d.ac.frequency)} />
            <Row label="Power factor" value={d.ac.powerFactor.toFixed(3)} />
            <Row label="Reactive power" value={`${d.ac.reactivePower.toFixed(0)} var`} />
            <Row label="Efficiency" value={`${d.ac.efficiency.toFixed(1)} %`} />
            <Row label="Peak power today" value={formatPower(d.ac.peakPower)} />
            <Row label="Insulation resistance" value={`${d.ac.insulation.toFixed(2)} MΩ`} />
          </Section>
        )}

        {/* Grid meter */}
        {d?.meter && (
          <Section icon={<Plug className="h-4 w-4" />} title="Grid Meter">
            <Row label="Phase A" value={`${v(d.meter.phaseAVoltage)} · ${a(d.meter.phaseACurrent)}`} />
            <Row label="Phase B" value={`${v(d.meter.phaseBVoltage)} · ${a(d.meter.phaseBCurrent)}`} />
            <Row label="Phase C" value={`${v(d.meter.phaseCVoltage)} · ${a(d.meter.phaseCCurrent)}`} />
            <Row label="Frequency" value={hz(d.meter.frequency)} />
            <Row label="Power factor" value={d.meter.powerFactor.toFixed(3)} />
            <Row label="Imported (total)" value={formatEnergy(d.meter.importedEnergy)} />
            <Row label="Exported (total)" value={formatEnergy(d.meter.exportedEnergy)} />
          </Section>
        )}

        {/* Battery */}
        {(status?.battery || d?.batteryDay) && (
          <Section icon={<BatteryCharging className="h-4 w-4" />} title="Battery">
            {status?.battery && (
              <>
                <Row label="State of charge" value={`${status.battery.soc.toFixed(0)} %`} />
                <Row
                  label={status.battery.power >= 0 ? 'Charging' : 'Discharging'}
                  value={formatPower(Math.abs(status.battery.power))}
                />
              </>
            )}
            {d?.batteryDay && (
              <>
                <Row label="Charged today" value={formatEnergy(d.batteryDay.charge)} />
                <Row label="Discharged today" value={formatEnergy(d.batteryDay.discharge)} />
              </>
            )}
          </Section>
        )}

        {!d && (
          <div className="text-center text-sm text-muted-foreground mt-8">
            No extended details available from this backend.
          </div>
        )}
      </div>
    </div>
  );
}
