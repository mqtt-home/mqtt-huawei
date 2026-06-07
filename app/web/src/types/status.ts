export interface BatteryStatus {
  soc: number;   // percent 0-100
  power: number; // watts, +charging / -discharging
}

export interface PVString {
  voltage: number; // V
  current: number; // A
  power: number;   // W
}

export interface ACDetail {
  phaseAVoltage: number;
  phaseBVoltage: number;
  phaseCVoltage: number;
  phaseACurrent: number;
  phaseBCurrent: number;
  phaseCCurrent: number;
  frequency: number;     // Hz
  powerFactor: number;
  efficiency: number;    // %
  reactivePower: number; // var
  peakPower: number;     // W (today's peak)
  insulation: number;    // MΩ
}

export interface MeterDetail {
  phaseAVoltage: number;
  phaseBVoltage: number;
  phaseCVoltage: number;
  phaseACurrent: number;
  phaseBCurrent: number;
  phaseCCurrent: number;
  frequency: number;      // Hz
  powerFactor: number;
  importedEnergy: number; // kWh
  exportedEnergy: number; // kWh
}

export interface BatteryDay {
  charge: number;    // kWh
  discharge: number; // kWh
}

export interface Details {
  pv?: PVString[];
  ac?: ACDetail;
  meter?: MeterDetail;
  batteryDay?: BatteryDay;
}

export interface InverterStatus {
  source: string;
  connected: boolean;
  model?: string;
  serial?: string;
  state?: string;
  pvPower: number;     // watts (DC input / solar generation)
  activePower: number; // watts (AC active power / inverter output)
  dailyYield: number;  // kWh today
  totalYield: number;  // kWh lifetime
  gridPower?: number;  // watts, +export / -import
  consumption?: number; // watts, estimated house load = activePower - gridPower (meter required)
  temperature?: number; // °C
  battery?: BatteryStatus;
  details?: Details;
  updatedAt: string;
}

// formatPower renders watts as W or kW with sensible precision.
export function formatPower(watts: number): string {
  const abs = Math.abs(watts);
  if (abs >= 1000) {
    return `${(watts / 1000).toFixed(2)} kW`;
  }
  return `${Math.round(watts)} W`;
}

// formatEnergy renders a kWh value with up to two decimals.
export function formatEnergy(kwh: number): string {
  if (kwh >= 1000) {
    return `${(kwh / 1000).toFixed(2)} MWh`;
  }
  return `${kwh.toFixed(2)} kWh`;
}
