# Theme System - Hướng dẫn sử dụng

## 1. Sử dụng Tailwind Classes (Khuyến nghị)

### Import theme utilities
```tsx
import { themeColors, themeClasses } from '~/styles/theme';
```

### Sử dụng các class có sẵn
```tsx
// Background
<div className={themeColors.bg.primary}>Primary background</div>
<div className={themeColors.bg.secondary}>Secondary background</div>

// Text
<p className={themeColors.text.primary}>Primary text</p>
<p className={themeColors.text.muted}>Muted text</p>

// Combined patterns
<div className={themeClasses.card}>Card với style đầy đủ</div>
<button className={themeClasses.buttonPrimary}>Primary Button</button>
```

## 2. Sử dụng CSS Variables

### Trong component
```tsx
import { cssVars } from '~/styles/theme';

<div style={{ backgroundColor: cssVars.background, color: cssVars.foreground }}>
  Content với CSS variables
</div>
```

### Trong CSS
```css
.my-custom-class {
  background-color: var(--color-background);
  color: var(--color-foreground);
  border-color: var(--color-border);
}
```

## 3. Sử dụng Hooks

```tsx
import { useThemeColors } from '~/hooks/useThemeColors';

function MyComponent() {
  const { isDark, isLight, getCSSVar, ifDark, ifLight } = useThemeColors();
  
  return (
    <div>
      <p>Current theme: {isDark ? 'Dark' : 'Light'}</p>
      <div className={ifDark('bg-gray-900')}>Only show in dark mode</div>
      <div className={ifLight('bg-white')}>Only show in light mode</div>
    </div>
  );
}
```

## 4. Các màu có sẵn

### Background Colors
- `themeColors.bg.primary` - Background chính
- `themeColors.bg.secondary` - Background phụ
- `themeColors.bg.tertiary` - Background cấp 3
- `themeColors.bg.card` - Background cho card
- `themeColors.bg.cardHover` - Hover state cho card

### Text Colors
- `themeColors.text.primary` - Text chính
- `themeColors.text.secondary` - Text phụ
- `themeColors.text.muted` - Text mờ/nhạt
- `themeColors.text.inverse` - Text ngược (trắng/đen)

### Border Colors
- `themeColors.border.primary` - Border chính
- `themeColors.border.secondary` - Border phụ
- `themeColors.border.light` - Border nhạt

### Brand Colors (Teal)
- `themeColors.brand.bg` - Background brand color
- `themeColors.brand.text` - Text brand color
- `themeColors.brand.border` - Border brand color

### Status Colors
- `themeColors.status.success` - Màu success
- `themeColors.status.warning` - Màu warning
- `themeColors.status.error` - Màu error
- `themeColors.status.info` - Màu info

### Interactive States
- `themeColors.interactive.hover` - Hover state
- `themeColors.interactive.active` - Active state
- `themeColors.interactive.focus` - Focus ring

## 5. Ví dụ thực tế

### Card component
```tsx
import { themeClasses, themeColors } from '~/styles/theme';

function MyCard() {
  return (
    <div className={themeClasses.card}>
      <h2 className={themeClasses.heading}>Title</h2>
      <p className={themeColors.text.secondary}>Description</p>
      <button className={themeClasses.buttonPrimary}>Action</button>
    </div>
  );
}
```

### Form input
```tsx
import { themeClasses } from '~/styles/theme';

function MyForm() {
  return (
    <input 
      type="text"
      className={themeClasses.input}
      placeholder="Enter text..."
    />
  );
}
```

### Custom styling với CSS variables
```tsx
import { cssVars } from '~/styles/theme';

function CustomComponent() {
  return (
    <div style={{
      backgroundColor: cssVars.card,
      borderColor: cssVars.border,
      boxShadow: cssVars.shadowMd,
      color: cssVars.foreground,
    }}>
      Content
    </div>
  );
}
```

## 6. Best Practices

1. **Ưu tiên dùng Tailwind classes** từ `themeColors` và `themeClasses`
2. **Dùng CSS variables** khi cần tính toán động hoặc inline styles
3. **Dùng hooks** khi cần logic conditional phức tạp
4. **Consistency** - Stick với một approach trong cùng một component
5. **Reusability** - Tạo components tái sử dụng thay vì copy/paste classes

## 7. Thêm màu mới

Để thêm màu mới, edit file `src/index.css`:

```css
:root {
  --color-custom: #yourcolor;
}

.dark {
  --color-custom: #yourdarkmodecolor;
}
```

Sau đó thêm vào `src/styles/theme.ts`:

```typescript
export const cssVars = {
  // ...existing vars
  custom: 'var(--color-custom)',
};

export const themeColors = {
  // ...existing colors
  custom: {
    text: 'text-[color:var(--color-custom)]',
    bg: 'bg-[color:var(--color-custom)]',
  },
};
```
