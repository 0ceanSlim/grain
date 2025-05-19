/** @type {import('tailwindcss').Config} */
module.exports = {
  content: ["./**/*.{html,js}"],
  theme: {
    extend: {
      colors: {
        bgPrimary: "var(--color-bgPrimary)",
        bgSecondary: "var(--color-bgSecondary)",
        bgInverted: "var(--color-bgInverted)",
        textPrimary: "var(--color-textPrimary)",
        textSecondary: "var(--color-textSecondary)",
        textMuted: "var(--color-textMuted)",
        textInverted: "var(--color-textInverted)",
      },
    },
  },
  plugins: [],
};
