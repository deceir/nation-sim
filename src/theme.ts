export type Theme='dark'|'light';
export function applyTheme(theme:Theme){document.documentElement.dataset.theme=theme;document.documentElement.style.colorScheme=theme;localStorage.setItem('diplomatia-theme',theme)}
export function storedTheme():Theme{return localStorage.getItem('diplomatia-theme')==='light'?'light':'dark'}
applyTheme(storedTheme());
