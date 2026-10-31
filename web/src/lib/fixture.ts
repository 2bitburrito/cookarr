import { NS_PER_MINUTE, type Recipe } from './types'

export const fixture: Recipe = {
  source_url: 'https://example.com/slow-tomato-soup',
  site_name: 'Example Kitchen',
  title: 'Slow-Roasted Tomato Soup',
  description: 'Sweet, jammy tomatoes roasted until they collapse, blitzed into a velvety soup.',
  yield: '4 bowls',
  prep_time: 15 * NS_PER_MINUTE,
  cook_time: 75 * NS_PER_MINUTE,
  total_time: 90 * NS_PER_MINUTE,
  keywords: ['soup', 'vegetarian', 'weeknight'],
  ingredients: [
    { group: 'Soup', text: '1.5 kg ripe tomatoes, halved' },
    { group: 'Soup', text: '1 head of garlic, top sliced off' },
    { group: 'Soup', text: '2 onions, quartered' },
    { group: 'Soup', text: '4 tbsp olive oil' },
    { group: 'Soup', text: '500 ml vegetable stock' },
    { group: 'Soup', text: '1 tsp sugar' },
    { group: 'Soup', text: 'Salt and black pepper' },
    { group: 'To serve', text: 'A handful of basil leaves' },
    { group: 'To serve', text: 'Crusty bread and butter' },
  ],
  steps: [
    { text: 'Heat the oven to 180°C. Spread the tomatoes, onions and garlic on a large tray, drizzle with oil and season well.' },
    { text: 'Roast for 60 minutes, until the tomatoes have collapsed and the edges are caramelised.' },
    { text: 'Squeeze the soft garlic out of its skins into a pot. Scrape in everything else from the tray, including the sticky bits.' },
    { text: 'Add the stock and sugar and simmer for 10 minutes.' },
    { text: 'Blitz until smooth, then taste and adjust the seasoning. Serve with torn basil and bread.' },
  ],
  story_html: `
    <p>My gran made this every October, when the greenhouse gave up and every last tomato had to be dealt with at once.</p>
    <p>The trick is patience. Roasting low and slow concentrates the sugars, so there's no need for cream, and the soup tastes like <em>more</em> tomato than the tomatoes did.</p>
    <p>It freezes well, so make a double batch.</p>`,
  photos: [
    { url: '', role: 'hero', caption: 'The finished soup' },
    { url: '', role: 'step', caption: 'Tomatoes, ready to roast' },
    { url: '', role: 'step', caption: 'Collapsed and caramelised' },
  ],
  author: {
    name: 'Example Cook',
    url: 'https://example.com/about',
    links: [
      { url: 'https://instagram.com/examplecook', kind: 'social', label: 'instagram' },
      { url: 'https://ko-fi.com/examplecook', kind: 'support', label: 'ko-fi' },
    ],
  },
  extractor: 'fixture',
}
