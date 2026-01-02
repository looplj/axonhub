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
