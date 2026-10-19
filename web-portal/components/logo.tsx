import Image from "next/image";

export function LogoMark({ size = 32, className }: { size?: number; className?: string }) {
  return <Image src="/logo-mark.svg" alt="" width={size} height={size} unoptimized className={className} style={{ width: size, height: size }} />;
}
