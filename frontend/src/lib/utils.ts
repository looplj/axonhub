import { type ClassValue, clsx } from 'clsx';
import { twMerge } from 'tailwind-merge';

export function cn(...inputs: ClassValue[]) {
  return twMerge(clsx(inputs));
}

export const extractNumberID = (id: string) => {
  const lastSlashIndex = id.lastIndexOf('/');
  return id.slice(lastSlashIndex + 1);
};

export const extractNumberIDAsNumber = (id: string) => {
  return Number(extractNumberID(id));
};

export const buildGUID = (type: string, id: string) => {
  return `gid://axonhub/${type}/${id}`;
};

const cjkCharacterPattern = /[\p{Script=Han}\p{Script=Hiragana}\p{Script=Katakana}\p{Script=Hangul}]/u;

// isCJKName reports whether any name contains a CJK script character.
export function isCJKName(...names: Array<string | null | undefined>) {
  return names.some((name) => !!name && cjkCharacterPattern.test(name));
}

// formatUserName returns CJK names as surname followed by given name and other names in Western order.
export function formatUserName(firstName?: string | null, lastName?: string | null) {
  const first = firstName?.trim() ?? '';
  const last = lastName?.trim() ?? '';

  if (first && last) {
    return isCJKName(first, last) ? `${last}${first}` : `${first} ${last}`;
  }

  return first || last;
}
