import { defineConfig } from 'astro/config';
import starlight from '@astrojs/starlight';

// https://starlight.astro.build/reference/configuration/
export default defineConfig({
  site: 'https://jbouhier.github.io',
  base: '/GoMessages/',
  integrations: [
    starlight({
      title: 'GoMessages',
      description: 'Google Messages for Mac, Windows, and Linux. A real desktop app.',
      logo: { src: './src/assets/icon.png', alt: 'GoMessages' },
      social: [{ icon: 'github', label: 'GitHub', href: 'https://github.com/jbouhier/GoMessages' }],
      sidebar: [
        { label: 'Download', items: ['downloads'] },
        { label: 'Start here', items: ['getting-started', 'pairing'] },
        { label: 'Using the app', items: ['shortcuts', 'settings', 'themes', 'troubleshooting'] },
        { label: 'Project', items: ['whats-new', 'dev-start'] },
      ],
    }),
  ],
});
