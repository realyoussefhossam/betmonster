"use client";

import { Button } from "@/components/ui/button";
import { signOut } from "@/lib/auth-client";
import { useRouter } from "next/navigation";
import { toast } from "sonner";

export const SignOutButton = ({ className }: { className?: string }) => {
  const router = useRouter();

  async function handleClick() {
    await signOut({
      fetchOptions: {
        onSuccess: () => {
          toast.success("Signed out");
          router.push("/login");
        },
        onError: (ctx) => {
          toast.error(ctx.error.message);
        },
      },
    });
  }

  return (
    <Button
      onClick={handleClick}
      size="sm"
      variant="destructive"
      className={className}
    >
      Sign Out
    </Button>
  );
};
