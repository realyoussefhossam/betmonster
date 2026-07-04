import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";

interface WalletCardProps {
  currency: string;
  balance: string;
  loading?: boolean;
}

export function WalletCard({ currency, balance, loading }: WalletCardProps) {
  return (
    <Card>
      <CardHeader>
        <CardTitle className="text-sm font-medium text-muted-foreground">
          {currency} Balance
        </CardTitle>
      </CardHeader>
      <CardContent>
        {loading ? (
          <Skeleton className="h-8 w-32" />
        ) : (
          <div className="text-2xl font-bold">{balance}</div>
        )}
      </CardContent>
    </Card>
  );
}
