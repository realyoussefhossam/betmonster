import Link from "next/link";
import { Button } from "@/components/ui/button";

export default function Page() {
  return (
    <div className="flex flex-col justify-center items-center h-screen gap-4">
      <h1 className="text-4xl font-bold">BetMonster</h1>
      <div className="flex gap-4">
        <Link href="/wallet">
          <Button>Wallet</Button>
        </Link>
        <Link href="/sportsbook">
          <Button variant="outline">Sportsbook</Button>
        </Link>
        <Link href="/admin/withdrawals">
          <Button variant="outline">Admin</Button>
        </Link>
      </div>
    </div>
  );
}
