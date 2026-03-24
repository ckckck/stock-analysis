const pad = (value: number): string => String(value).padStart(2, '0');

const parseDateLike = (value?: string): Date | null => {
  if (!value) return null;
  const trimmed = value.trim();
  if (!trimmed) return null;

  if (/^\d{4}-\d{2}-\d{2}$/.test(trimmed)) {
    const parsed = new Date(`${trimmed}T00:00:00`);
    return Number.isNaN(parsed.getTime()) ? null : parsed;
  }

  const parsed = new Date(trimmed);
  return Number.isNaN(parsed.getTime()) ? null : parsed;
};

export const formatDateTimeDisplay = (value?: string, fallback = '--'): string => {
  const parsed = parseDateLike(value);
  if (!parsed) return value?.trim() || fallback;

  return [
    `${parsed.getFullYear()}-${pad(parsed.getMonth() + 1)}-${pad(parsed.getDate())}`,
    `${pad(parsed.getHours())}:${pad(parsed.getMinutes())}:${pad(parsed.getSeconds())}`,
  ].join(' ');
};

export const formatDateDisplay = (value?: string, fallback = '--'): string => {
  const parsed = parseDateLike(value);
  if (!parsed) return value?.trim() || fallback;
  return `${parsed.getFullYear()}-${pad(parsed.getMonth() + 1)}-${pad(parsed.getDate())}`;
};
