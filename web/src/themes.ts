// Theme system (task R14). A theme is a named set of CSS custom properties.
// Two builtin themes (Light, Dark) always exist; users can create custom
// themes and select any theme as the one used for light mode and for dark
// mode — the main-window toggle then switches between those two selections.
// Themes and selections persist in localStorage so they work identically in
// the Wails webview and a browser.

export type ThemeMode = 'light' | 'dark';

export interface ThemeTokens {
  bg: string;
  text: string;
  muted: string;
  panelBg: string;
  border: string;
  borderStrong: string;
  accent: string;
  accentSoft: string;
  hover: string;
  chipBg: string;
}

export interface Theme {
  name: string;
  base: ThemeMode; // which editor/preview theme family it belongs to
  tokens: ThemeTokens;
  builtin?: boolean;
}

export const lightTheme: Theme = {
  name: 'Light',
  base: 'light',
  builtin: true,
  tokens: {
    bg: '#f7f7fb',
    text: '#172033',
    muted: '#5b6472',
    panelBg: '#ffffff',
    border: '#e5e7eb',
    borderStrong: '#d9dee7',
    accent: '#2563eb',
    accentSoft: '#d7e3f7',
    hover: '#e8edf5',
    chipBg: '#e8edf5',
  },
};

export const darkTheme: Theme = {
  name: 'Dark',
  base: 'dark',
  builtin: true,
  tokens: {
    bg: '#0f172a',
    text: '#e2e8f0',
    muted: '#94a3b8',
    panelBg: '#1e293b',
    border: '#334155',
    borderStrong: '#3b4a63',
    accent: '#60a5fa',
    accentSoft: '#1d3a6b',
    hover: '#26334d',
    chipBg: '#26334d',
  },
};

const CUSTOM_KEY = 'notrios.customThemes';
const MODE_KEY = 'notrios.themeMode';
const LIGHT_CHOICE_KEY = 'notrios.lightTheme';
const DARK_CHOICE_KEY = 'notrios.darkTheme';

export function loadCustomThemes(): Theme[] {
  try {
    const raw = localStorage.getItem(CUSTOM_KEY);
    if (!raw) return [];
    const parsed = JSON.parse(raw) as Theme[];
    return parsed.filter((theme) => theme && theme.name && theme.tokens);
  } catch {
    return [];
  }
}

export function saveCustomThemes(themes: Theme[]) {
  localStorage.setItem(CUSTOM_KEY, JSON.stringify(themes));
}

export function allThemes(custom: Theme[]): Theme[] {
  return [lightTheme, darkTheme, ...custom];
}

export function loadMode(): ThemeMode {
  return localStorage.getItem(MODE_KEY) === 'dark' ? 'dark' : 'light';
}

export function saveMode(mode: ThemeMode) {
  localStorage.setItem(MODE_KEY, mode);
}

// loadChoice returns the theme name selected for a mode ("Light"/"Dark" by default).
export function loadChoice(mode: ThemeMode): string {
  const value = localStorage.getItem(mode === 'light' ? LIGHT_CHOICE_KEY : DARK_CHOICE_KEY);
  return value || (mode === 'light' ? lightTheme.name : darkTheme.name);
}

export function saveChoice(mode: ThemeMode, themeName: string) {
  localStorage.setItem(mode === 'light' ? LIGHT_CHOICE_KEY : DARK_CHOICE_KEY, themeName);
}

export function resolveTheme(mode: ThemeMode, custom: Theme[]): Theme {
  const wanted = loadChoice(mode);
  const found = allThemes(custom).find((theme) => theme.name === wanted);
  return found ?? (mode === 'light' ? lightTheme : darkTheme);
}

// applyTheme writes the theme tokens onto <html> so every var() in styles.css
// picks them up.
export function applyTheme(theme: Theme) {
  const root = document.documentElement;
  const tokens = theme.tokens;
  root.style.setProperty('--bg', tokens.bg);
  root.style.setProperty('--text', tokens.text);
  root.style.setProperty('--muted', tokens.muted);
  root.style.setProperty('--panel-bg', tokens.panelBg);
  root.style.setProperty('--border', tokens.border);
  root.style.setProperty('--border-strong', tokens.borderStrong);
  root.style.setProperty('--accent', tokens.accent);
  root.style.setProperty('--accent-soft', tokens.accentSoft);
  root.style.setProperty('--hover', tokens.hover);
  root.style.setProperty('--chip-bg', tokens.chipBg);
  root.dataset.themeBase = theme.base;
}
