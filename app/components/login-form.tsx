"use client";

import { Label } from "@/components/ui/label";
import { Input } from "@/components/ui/input";
import { Button } from "@/components/ui/button";
import { toast } from "sonner";
import { signIn } from "@/lib/auth-client";
import { useRouter } from "next/navigation";
import Link from "next/link";

export const LoginForm = () => {
  const router = useRouter();

  async function handleSubmit(evt: React.FormEvent<HTMLFormElement>) {
    evt.preventDefault();

    const formData = new FormData(evt.currentTarget);

    const email = String(formData.get("email"));
    if (!email) return toast.error("Please enter your email");

    const password = String(formData.get("password"));
    if (!password) return toast.error("Please enter your password");

    await signIn.email(
      { email, password },
      {
        onRequest: () => {
          toast.loading("Logging in...", { id: "login" });
        },
        onSuccess: () => {
          toast.success("Welcome back", { id: "login" });
          router.push("/profile");
        },
        onError: (ctx) => {
          toast.error(ctx.error.message, { id: "login" });
        },
      },
    );
  }

  return (
    <form onSubmit={handleSubmit} className="max-w-sm w-full space-y-4">
      <div className="space-y-2">
        <Label htmlFor="email">Email</Label>
        <Input type="email" id="email" name="email" placeholder="Enter your email" />
      </div>

      <div className="space-y-2">
        <Label htmlFor="password">Password</Label>
        <Input type="password" id="password" name="password" placeholder="Enter your password" />
      </div>

      <Button type="submit" className="w-full">
        Login
      </Button>

      <p className="text-sm text-center text-muted-foreground">
        {"Don't have an account? "}
        <Link href="/register" className="text-primary hover:underline">
          Register
        </Link>
      </p>
    </form>
  );
};
