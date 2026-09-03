const serviceImages = {
  nursing: "/images/brand/portraits/theresa-yirerong-by-jinnifer-douglass-037.webp",
  coaching: "/images/brand/portraits/theresa-yirerong-by-jinnifer-douglass-062.webp",
  mindfulness: "/images/blog/lavender-3605688_1280.webp",
  nutrition: "/images/blog/meal-2834549_1280.webp",
  bodywork: "/images/blog/swan-2077219_1280.webp",
  conversation: "/images/brand/portraits/theresa-yirerong-by-jinnifer-douglass-010.webp",
} as const;

const fallbackImages = [
  serviceImages.coaching,
  serviceImages.nursing,
  serviceImages.mindfulness,
  serviceImages.nutrition,
] as const;

/** Gives every live service warm, relevant imagery without requiring a schema
 * migration. Known care themes receive a semantic image; custom services use
 * a stable rotation so the catalog never falls back to a text-only row. */
export function serviceImageFor(name: string, index = 0): string {
  const normalized = name.toLowerCase();
  if (/intro|conversation|discovery|first session/.test(normalized)) return serviceImages.conversation;
  if (/nurs|clinical|health consultation/.test(normalized)) return serviceImages.nursing;
  if (/mind|meditat|rest|recover|stress|sleep/.test(normalized)) return serviceImages.mindfulness;
  if (/massage|bodywork|relax|spa/.test(normalized)) return serviceImages.bodywork;
  if (/nutri|food|meal|nourish/.test(normalized)) return serviceImages.nutrition;
  if (/coach|wellness|wellbeing|holistic/.test(normalized)) return serviceImages.coaching;
  return fallbackImages[index % fallbackImages.length];
}
