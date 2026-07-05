import { Card, CardContent } from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";

export interface WalletCardProps {
  currency: string;
  balance: string;
  fiatValue?: string;
  fiatCurrency?: string;
  loading?: boolean;
}

export function WalletCard({
  currency,
  balance,
  fiatValue,
  fiatCurrency,
  loading,
}: WalletCardProps) {
  return (
    <Card>
      <CardContent className="p-6">
        {loading ? (
          <Skeleton className="h-6 w-24" />
        ) : (
          <>
            <div className="text-2xl font-bold">
              {balance} {currency}
            </div>
            {fiatValue && fiatCurrency && (
              <div className="text-sm text-muted-foreground">
                ≈ ${fiatValue} {fiatCurrency}
              </div>
            )}
          </>
        )}
      </CardContent>
    </Card>
  );
}
