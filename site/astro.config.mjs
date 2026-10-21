import { defineConfig } from 'astro/config';
import starlight from '@astrojs/starlight';

const site = 'https://jbouhier.github.io';
const base = '/GoMessages/';

// https://starlight.astro.build/reference/configuration/
export default defineConfig({
  site,
  base,
  integrations: [
    starlight({
      title: 'GoMessages',
      description: 'Google Messages for Mac, Windows, and Linux. A real desktop app.',
      logo: { src: './src/assets/icon.png', alt: 'GoMessages' },
      favicon: '/favicon.png',
      social: [{ icon: 'github', label: 'GitHub', href: 'https://github.com/jbouhier/GoMessages' }],
      customCss: ['@fontsource-variable/inter/opsz.css', './src/styles/theme.css'],
      components: {
        Hero: './src/components/Hero.astro',
        SocialIcons: './src/components/SocialIcons.astro',
      },
      expressiveCode: {
        styleOverrides: {
          borderRadius: '12px',
          borderColor: 'var(--sl-color-hairline-light)',
          codeFontSize: '0.8125rem',
          frames: { shadowColor: 'transparent' },
        },
      },
      head: [
        { tag: 'meta', attrs: { property: 'og:image', content: `${site}${base}product/social-card.jpg` } },
        { tag: 'meta', attrs: { name: 'twitter:card', content: 'summary_large_image' } },
        { tag: 'meta', attrs: { name: 'theme-color', content: '#ffffff', media: '(prefers-color-scheme: light)' } },
        { tag: 'meta', attrs: { name: 'theme-color', content: '#131314', media: '(prefers-color-scheme: dark)' } },
      ],
      sidebar: [
        { label: 'Download', items: ['downloads'] },
        { label: 'Start here', items: ['getting-started', 'pairing'] },
        { label: 'Using the app', items: ['shortcuts', 'settings', 'themes', 'troubleshooting'] },
        { label: 'Project', items: ['whats-new', 'dev-start'] },
      ],
    }),
  ],
});
