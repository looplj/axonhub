import React, { createContext, useContext, useEffect, useState } from 'react';
import { sansFonts, serifFonts, monoFonts, fontStacks, systemStacks } from '@/config/fonts';

type SansFont = (typeof sansFonts)[number];
type SerifFont = (typeof serifFonts)[number];
type MonoFont = (typeof monoFonts)[number];

type FontCategory = 'sans' | 'serif' | 'mono';

interface FontContextType {
  sansFont: SansFont;
  serifFont: SerifFont;
  monoFont: MonoFont;
  setSansFont: (font: SansFont) => void;
  setSerifFont: (font: SerifFont) => void;
  setMonoFont: (font: MonoFont) => void;
}

const FONT_VAR: Record<FontCategory, string> = {
  sans: '--font-sans',
  serif: '--font-serif',
  mono: '--font-mono',
};

const FONT_KEY: Record<FontCategory, string> = {
  sans: 'font-sans',
  serif: 'font-serif',
  mono: 'font-mono',
};

function readStored(key: string, options: readonly string[], fallback: string): string {
  const saved = localStorage.getItem(key);
  return options.includes(saved ?? '') ? (saved as string) : fallback;
}

const FontContext = createContext<FontContextType | undefined>(undefined);

export const FontProvider: React.FC<{ children: React.ReactNode }> = ({ children }) => {
  const [sansFont, _setSansFont] = useState<SansFont>(() => readStored(FONT_KEY.sans, sansFonts, 'theme') as SansFont);
  const [serifFont, _setSerifFont] = useState<SerifFont>(() => readStored(FONT_KEY.serif, serifFonts, 'theme') as SerifFont);
  const [monoFont, _setMonoFont] = useState<MonoFont>(() => readStored(FONT_KEY.mono, monoFonts, 'theme') as MonoFont);

  useEffect(() => {
    const root = document.documentElement;

    const applyFont = (category: FontCategory, font: string) => {
      const cssVar = FONT_VAR[category];
      if (font === 'theme') {
        // 'theme'：跟随主题默认字体，移除覆盖让主题类生效
        root.style.removeProperty(cssVar);
      } else if (font === 'system') {
        // 'system'：直接使用操作系统默认字体，不跟随主题
        root.style.setProperty(cssVar, systemStacks[category]);
      } else {
        root.style.setProperty(cssVar, fontStacks[font] ?? font);
      }
    };

    applyFont('sans', sansFont);
    applyFont('serif', serifFont);
    applyFont('mono', monoFont);
  }, [sansFont, serifFont, monoFont]);

  const setSansFont = (font: SansFont) => {
    localStorage.setItem(FONT_KEY.sans, font);
    _setSansFont(font);
  };
  const setSerifFont = (font: SerifFont) => {
    localStorage.setItem(FONT_KEY.serif, font);
    _setSerifFont(font);
  };
  const setMonoFont = (font: MonoFont) => {
    localStorage.setItem(FONT_KEY.mono, font);
    _setMonoFont(font);
  };

  return (
    <FontContext value={{ sansFont, serifFont, monoFont, setSansFont, setSerifFont, setMonoFont }}>
      {children}
    </FontContext>
  );
};

// eslint-disable-next-line react-refresh/only-export-components
export const useFont = () => {
  const context = useContext(FontContext);
  if (!context) {
    throw new Error('useFont must be used within a FontProvider');
  }
  return context;
};
