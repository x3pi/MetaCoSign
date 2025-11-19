import React from "react";
import { cn } from "~/lib/utils";

interface PageContainerProps {
  children: React.ReactNode;
  className?: string;
  maxWidth?: "sm" | "md" | "lg" | "xl" | "2xl" | "4xl";
}

export function PageContainer({
  children,
  className,
  maxWidth = "4xl",
}: PageContainerProps) {
  const maxWidthClasses = {
    sm: "max-w-sm",
    md: "max-w-md",
    lg: "max-w-lg",
    xl: "max-w-xl",
    "2xl": "max-w-2xl",
    "4xl": "max-w-4xl",
  };

  return (
    <div className="min-h-screen bg-linear-to-b from-(--color-background-secondary) via-(--color-background-secondary) to-(--color-background-tertiary) p-4 md:p-8 transition-colors duration-300">
      <div
        className={cn(
          "mx-auto space-y-6",
          maxWidthClasses[maxWidth],
          className
        )}
      >
        {children}
      </div>
    </div>
  );
}
