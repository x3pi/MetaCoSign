import { forwardRef } from "react";
import { Button, type ButtonProps } from "./button";
import { cn } from "~/lib/utils";

/**
 * AppButton - Button component với styles tối ưu cho cả Light/Dark mode
 * Sử dụng CSS variables từ index.css để dễ dàng tùy chỉnh màu sắc
 */

export interface AppButtonProps extends ButtonProps {
  /**
   * Variant của button
   * - primary: Button chính (xanh dương)
   * - success: Button thành công (xanh lá)
   * - danger: Button nguy hiểm (đỏ)
   * - outline: Button viền
   * - ghost: Button trong suốt
   */
  appVariant?: "primary" | "success" | "danger" | "outline" | "ghost";
}

const AppButton = forwardRef<HTMLButtonElement, AppButtonProps>(
  ({ className, appVariant = "outline", disabled, ...props }, ref) => {
    // Base styles cho tất cả buttons
    const baseStyles = "transition-all duration-200 font-medium";

    // Variant styles - MIX TAILWIND + CSS VARIABLES (KHÔNG DÙNG dark:)
    const variantStyles = {
      primary: cn(
        "bg-primary hover:bg-primary-hover text-white",
        "disabled:opacity-50 disabled:cursor-not-allowed",
        "shadow-sm hover:shadow-md"
      ),
      success: cn(
        "bg-success text-white hover:brightness-110",
        "disabled:opacity-50 disabled:cursor-not-allowed",
        "shadow-sm hover:shadow-md"
      ),
      danger: cn(
        "bg-error text-white hover:brightness-110",
        "disabled:opacity-50 disabled:cursor-not-allowed",
        "shadow-sm hover:shadow-md"
      ),
      outline: cn(
        "border-2 border-border bg-card text-foreground",
        "hover:bg-card-hover",
        "disabled:opacity-50 disabled:cursor-not-allowed"
      ),
      ghost: cn(
        "bg-transparent text-foreground hover:bg-card-hover",
        "disabled:opacity-50 disabled:cursor-not-allowed"
      ),
    };

    return (
      <Button
        ref={ref}
        className={cn(baseStyles, variantStyles[appVariant], className)}
        disabled={disabled}
        {...props}
      />
    );
  }
);

AppButton.displayName = "AppButton";

export { AppButton };
