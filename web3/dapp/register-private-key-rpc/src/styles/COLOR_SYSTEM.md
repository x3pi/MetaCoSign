# Hệ thống màu sắc tái sử dụng

## 🎨 Cách hoạt động

Tất cả màu sắc được định nghĩa ở **1 NƠI DUY NHẤT**: `src/index.css`

Khi bạn đổi màu trong `index.css`, **TẤT CẢ** component sử dụng màu đó sẽ tự động thay đổi!

## 📝 Cách sử dụng

### Cách 1: Dùng CSS Classes (Đơn giản nhất)

```tsx
import { AppColorClasses } from '~/styles/colors';

function MyComponent() {
  return (
    <div className="bg-app text-app">
      <h1 className="text-primary">Title with brand color</h1>
      <p className="text-app-secondary">Secondary text</p>
      
      <button className="bg-primary text-white hover:bg-primary-hover">
        Button
      </button>
    </div>
  );
}
```

### Cách 2: Dùng CSS Variables trong inline styles

```tsx
import { AppColors } from '~/styles/colors';

function MyComponent() {
  return (
    <div style={{ 
      backgroundColor: AppColors.background,
      color: AppColors.foreground,
      borderColor: AppColors.border
    }}>
      Content
    </div>
  );
}
```

### Cách 3: Dùng trong Tailwind với arbitrary values

```tsx
<div className="bg-[var(--color-primary)] text-[var(--color-foreground)]">
  Content
</div>
```

## 🔧 Đổi màu toàn bộ app

### Ví dụ: Đổi brand color từ Teal → Blue

Mở file `src/index.css` và sửa:

```css
:root {
  /* Trước: Teal */
  --color-primary: #0891b2;
  
  /* Sau: Blue */  
  --color-primary: #3b82f6; /* blue-600 */
}

.dark {
  /* Trước: Teal */
  --color-primary: #14b8a6;
  
  /* Sau: Blue */
  --color-primary: #60a5fa; /* blue-400 */
}
```

**Xong!** Tất cả button, link, icon dùng brand color sẽ tự động đổi sang màu blue! 🎉

## 📋 Danh sách màu có sẵn

### CSS Classes
- `bg-app` - Background chính
- `bg-app-secondary` - Background phụ
- `text-app` - Text chính
- `text-app-secondary` - Text phụ
- `text-app-muted` - Text mờ
- `bg-primary` - Brand color background
- `bg-primary-hover` - Brand color hover
- `text-primary` - Brand color text
- `border-primary` - Border chính
- `text-success` / `bg-success` - Success color
- `text-error` / `bg-error` - Error color

### CSS Variables (cho inline styles)
- `var(--color-primary)` - Brand color
- `var(--color-background)` - Background
- `var(--color-foreground)` - Text color
- `var(--color-border)` - Border color
- `var(--color-success)` - Success color
- `var(--color-error)` - Error color
- `var(--color-warning)` - Warning color

## 💡 Best Practices

### ✅ NÊN:
```tsx
// Dùng CSS classes
<button className="bg-primary hover:bg-primary-hover">Click</button>

// Hoặc CSS variables
<div style={{ color: AppColors.primary }}>Text</div>
```

### ❌ KHÔNG NÊN:
```tsx
// Hard-code màu trực tiếp
<button className="bg-teal-600 hover:bg-teal-700">Click</button>

// Inline color values
<div style={{ color: '#0891b2' }}>Text</div>
```

### Tại sao?
- ✅ Dễ maintain - Đổi 1 chỗ, ảnh hưởng tất cả
- ✅ Consistent - Đảm bảo màu giống nhau khắp nơi
- ✅ Dark mode tự động - Không cần viết `dark:` mỗi lần
- ✅ Flexible - Dễ thử nghiệm với màu khác nhau

## 🎯 Ví dụ thực tế

### Trước (Hard-coded):
```tsx
<button className="bg-teal-600 hover:bg-teal-700 dark:bg-teal-500 dark:hover:bg-teal-600">
  Submit
</button>

<button className="bg-purple-600 hover:bg-purple-700 dark:bg-purple-500 dark:hover:bg-purple-600">
  Cancel  
</button>
```

**Vấn đề**: Nếu muốn đổi tất cả button sang màu blue, phải tìm và sửa từng nơi! 😫

### Sau (Dùng CSS variables):
```tsx
<button className="bg-primary hover:bg-primary-hover">
  Submit
</button>

<button className="bg-primary hover:bg-primary-hover">
  Cancel
</button>
```

**Lợi ích**: Chỉ cần đổi `--color-primary` trong `index.css`, TẤT CẢ button đổi màu! 🎉

## 🔄 Migration Plan

Để chuyển đổi code hiện tại:

1. ✅ Đã setup CSS variables và classes trong `index.css`
2. ✅ Đã tạo `colors.ts` để reference dễ dàng
3. 🔄 **TODO**: Refactor các component để dùng system này

### Ví dụ refactor:

**Trước:**
```tsx
<div className="bg-white dark:bg-neutral-900">
  <h1 className="text-neutral-900 dark:text-neutral-100">Title</h1>
</div>
```

**Sau:**
```tsx
<div className="bg-app">
  <h1 className="text-app">Title</h1>
</div>
```

Ngắn gọn, clean và dễ maintain hơn nhiều!
