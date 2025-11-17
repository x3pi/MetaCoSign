import * as React from "react";
import { cva, type VariantProps } from "class-variance-authority";
import { cn } from "~/lib/utils";
import { AlertCircle, CheckCircle2, Info, XCircle } from "lucide-react";

const alertVariants = cva(
  "relative w-full rounded-lg border p-4 [&>svg~*]:pl-7 [&>svg+div]:translate-y-[-3px] [&>svg]:absolute [&>svg]:left-4 [&>svg]:top-4 [&>svg]:text-neutral-900 dark:[&>svg]:text-neutral-100",
  {
    variants: {
      variant: {
        default:
          "bg-neutral-100 dark:bg-neutral-800 text-neutral-900 dark:text-neutral-100 border-neutral-300 dark:border-neutral-600",
        destructive:
          "border-red-300 dark:border-red-700/60 bg-red-50 dark:bg-red-800/40 text-red-900 dark:text-red-300 [&>svg]:text-red-600 dark:[&>svg]:text-red-400",
        success:
          "border-green-300 dark:border-green-700/60 bg-green-50 dark:bg-green-800/40 text-green-900 dark:text-green-300 [&>svg]:text-green-600 dark:[&>svg]:text-green-400",
        warning:
          "border-yellow-300 dark:border-yellow-700/60 bg-yellow-50 dark:bg-yellow-800/40 text-yellow-900 dark:text-yellow-300 [&>svg]:text-yellow-600 dark:[&>svg]:text-yellow-400",
        info: "border-sky-300 dark:border-sky-700/60 bg-sky-50 dark:bg-sky-800/40 text-sky-900 dark:text-sky-300 [&>svg]:text-sky-600 dark:[&>svg]:text-sky-400",
      },
    },
    defaultVariants: {
      variant: "default",
    },
  }
);

const Alert = React.forwardRef<
  HTMLDivElement,
  React.HTMLAttributes<HTMLDivElement> &
    VariantProps<typeof alertVariants> & { icon?: boolean }
>(({ className, variant, icon = true, children, ...props }, ref) => {
  const Icon =
    variant === "destructive"
      ? XCircle
      : variant === "success"
      ? CheckCircle2
      : variant === "warning"
      ? AlertCircle
      : variant === "info"
      ? Info
      : AlertCircle;

  return (
    <div
      ref={ref}
      role="alert"
      className={cn(alertVariants({ variant }), className)}
      {...props}
    >
      {icon && <Icon className="h-4 w-4" />}
      {children}
    </div>
  );
});
Alert.displayName = "Alert";

const AlertTitle = React.forwardRef<
  HTMLParagraphElement,
  React.HTMLAttributes<HTMLHeadingElement>
>(({ className, ...props }, ref) => (
  <h5
    ref={ref}
    className={cn("mb-1 font-medium leading-none tracking-tight", className)}
    {...props}
  />
));
AlertTitle.displayName = "AlertTitle";

const AlertDescription = React.forwardRef<
  HTMLParagraphElement,
  React.HTMLAttributes<HTMLParagraphElement>
>(({ className, ...props }, ref) => (
  <div
    ref={ref}
    className={cn("text-sm [&_p]:leading-relaxed", className)}
    {...props}
  />
));
AlertDescription.displayName = "AlertDescription";

export { Alert, AlertTitle, AlertDescription };
