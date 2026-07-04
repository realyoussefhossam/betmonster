declare module "lucide-react" {
  import type { SVGProps } from "react";

  type IconProps = SVGProps<SVGSVGElement>;

  export const CircleCheckIcon: React.FC<IconProps>;
  export const InfoIcon: React.FC<IconProps>;
  export const TriangleAlertIcon: React.FC<IconProps>;
  export const OctagonXIcon: React.FC<IconProps>;
  export const Loader2Icon: React.FC<IconProps>;
  export const LoaderIcon: React.FC<IconProps>;
  export const ArrowLeftIcon: React.FC<IconProps>;

  const LucideIcon: React.FC<IconProps>;
  export default LucideIcon;
}
