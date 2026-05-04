interface SpinnerProps {
  size?: "sm" | "md" | "lg";
}

const sizeClass = {
  sm: "h-4 w-4 border-2",
  md: "h-8 w-8 border-2",
  lg: "h-12 w-12 border-4",
};

export function Spinner({ size = "md" }: SpinnerProps) {
  return (
    <div className="flex items-center justify-center">
      <div
        className={`${sizeClass[size]} animate-spin rounded-full border-gray-200 border-t-indigo-600`}
      />
    </div>
  );
}
