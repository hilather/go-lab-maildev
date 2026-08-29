const MONTHS = ["Jan", "Feb", "Mar", "Apr", "May", "Jun", "Jul", "Aug", "Sep", "Oct", "Nov", "Dec"] as const;

function pad2(n: number): string {
  return String(n).padStart(2, "0");
}

function sameLocalDay(a: Date, b: Date): boolean {
  return a.getFullYear() === b.getFullYear() && a.getMonth() === b.getMonth() && a.getDate() === b.getDate();
}

export function formatRelativeReceived(iso: string, now: Date = new Date()): string {
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) {
    return iso;
  }
  if (sameLocalDay(d, now)) {
    return `${pad2(d.getHours())}:${pad2(d.getMinutes())}`;
  }
  const day = `${d.getDate()} ${MONTHS[d.getMonth()]}`;
  if (d.getFullYear() === now.getFullYear()) {
    return day;
  }
  return `${day} ${d.getFullYear()}`;
}
