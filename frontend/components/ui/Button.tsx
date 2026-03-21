import { cn } from '@/lib/utils';
import { ButtonHTMLAttributes } from 'react';

type Variant = 'primary' | 'secondary' | 'ghost' | 'danger';
type Size    = 'sm' | 'md';

interface ButtonProps extends ButtonHTMLAttributes<HTMLButtonElement> {
  variant?: Variant;
  size?: Size;
}

const VARIANT: Record<Variant, string> = {
  primary:   'bg-violet-600 hover:bg-violet-700 text-white border-transparent shadow-sm',
  secondary: 'bg-white hover:bg-slate-50 text-slate-700 border-slate-300 shadow-sm',
  ghost:     'bg-transparent hover:bg-slate-100 text-slate-600 hover:text-slate-900 border-transparent',
  danger:    'bg-white hover:bg-red-50 text-red-600 border-red-300 shadow-sm',
};

const SIZE: Record<Size, string> = {
  sm: 'px-3 py-1.5 text-xs',
  md: 'px-4 py-2 text-sm',
};

export function Button({ variant = 'primary', size = 'md', className, children, disabled, ...props }: ButtonProps) {
  return (
    <button
      {...props}
      disabled={disabled}
      className={cn(
        'inline-flex items-center gap-1.5 rounded-md border font-medium transition-colors',
        'disabled:opacity-50 disabled:cursor-not-allowed',
        VARIANT[variant],
        SIZE[size],
        className,
      )}
    >
      {children}
    </button>
  );
}
