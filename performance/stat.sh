#!/bin/bash
awk -F "\t" '{print $3}' dmmspot.log|grep 'futuresBookTicker' | wc -l
awk -F "\t" '{print $3}' dmmspot.log|grep 'futuresBookTicker' |awk -F '|' '{if($NF<=3 ){sum+=1}} END {print sum}';
awk -F "\t" '{print $3}' dmmspot.log|grep 'futuresBookTicker' |awk -F '|' '{if($NF>3 && $NF <= 10 ){sum+=1}} END {print sum}';
awk -F "\t" '{print $3}' dmmspot.log|grep 'futuresBookTicker' |awk -F '|' '{if($NF>10 && $NF <= 100 ){sum+=1}} END {print sum}';
awk -F "\t" '{print $3}' dmmspot.log|grep 'futuresBookTicker' |awk -F '|' '{if($NF>100 && $NF <= 500 ){sum+=1}} END {print sum}';
awk -F "\t" '{print $3}' dmmspot.log|grep 'futuresBookTicker' |awk -F '|' '{if($NF>500 && $NF <= 1000 ){sum+=1}} END {print sum}';
awk -F "\t" '{print $3}' dmmspot.log|grep 'futuresBookTicker' |awk -F '|' '{if($NF>1000 ){sum+=1}} END {print sum}';

awk -F "\t" '{print $3}' dmmspot.log|grep 'deliveryBookTicker' | wc -l
awk -F "\t" '{print $3}' dmmspot.log|grep 'deliveryBookTicker' |awk -F '|' '{if($NF<=3 ){sum+=1}} END {print sum}';
awk -F "\t" '{print $3}' dmmspot.log|grep 'deliveryBookTicker' |awk -F '|' '{if($NF>3 && $NF <= 10 ){sum+=1}} END {print sum}';
awk -F "\t" '{print $3}' dmmspot.log|grep 'deliveryBookTicker' |awk -F '|' '{if($NF>10 && $NF <= 100 ){sum+=1}} END {print sum}';
awk -F "\t" '{print $3}' dmmspot.log|grep 'deliveryBookTicker' |awk -F '|' '{if($NF>100 && $NF <= 500 ){sum+=1}} END {print sum}';
awk -F "\t" '{print $3}' dmmspot.log|grep 'deliveryBookTicker' |awk -F '|' '{if($NF>500 && $NF <= 1000 ){sum+=1}} END {print sum}';
awk -F "\t" '{print $3}' dmmspot.log|grep 'deliveryBookTicker' |awk -F '|' '{if($NF>1000 ){sum+=1}} END {print sum}';

awk -F "\t" '{print $3}' dmmspot.log|grep 'spotBookTicker' | wc -l
awk -F "\t" '{print $3}' dmmspot.log|grep 'spotBookTicker' |awk -F '|' '{if($NF<=3 ){sum+=1}} END {print sum}';
awk -F "\t" '{print $3}' dmmspot.log|grep 'spotBookTicker' |awk -F '|' '{if($NF>3 && $NF <= 10 ){sum+=1}} END {print sum}';
awk -F "\t" '{print $3}' dmmspot.log|grep 'spotBookTicker' |awk -F '|' '{if($NF>10 && $NF <= 100 ){sum+=1}} END {print sum}';
awk -F "\t" '{print $3}' dmmspot.log|grep 'spotBookTicker' |awk -F '|' '{if($NF>100 && $NF <= 500 ){sum+=1}} END {print sum}';
awk -F "\t" '{print $3}' dmmspot.log|grep 'spotBookTicker' |awk -F '|' '{if($NF>500 && $NF <= 1000 ){sum+=1}} END {print sum}';
awk -F "\t" '{print $3}' dmmspot.log|grep 'spotBookTicker' |awk -F '|' '{if($NF>1000 ){sum+=1}} END {print sum}';

awk -F "\t" '{print $3}' dmmspot.log|grep 'futuresDepth' | wc -l
awk -F "\t" '{print $3}' dmmspot.log|grep 'futuresDepth' |awk -F '|' '{if($NF<=3 ){sum+=1}} END {print sum}';
awk -F "\t" '{print $3}' dmmspot.log|grep 'futuresDepth' |awk -F '|' '{if($NF>3 && $NF <= 10 ){sum+=1}} END {print sum}';
awk -F "\t" '{print $3}' dmmspot.log|grep 'futuresDepth' |awk -F '|' '{if($NF>10 && $NF <= 100 ){sum+=1}} END {print sum}';
awk -F "\t" '{print $3}' dmmspot.log|grep 'futuresDepth' |awk -F '|' '{if($NF>100 && $NF <= 500 ){sum+=1}} END {print sum}';
awk -F "\t" '{print $3}' dmmspot.log|grep 'futuresDepth' |awk -F '|' '{if($NF>500 && $NF <= 1000 ){sum+=1}} END {print sum}';
awk -F "\t" '{print $3}' dmmspot.log|grep 'futuresDepth' |awk -F '|' '{if($NF>1000 ){sum+=1}} END {print sum}';

awk -F "\t" '{print $3}' dmmspot.log|grep 'deliveryDepth' | wc -l
awk -F "\t" '{print $3}' dmmspot.log|grep 'deliveryDepth' |awk -F '|' '{if($NF<=3 ){sum+=1}} END {print sum}';
awk -F "\t" '{print $3}' dmmspot.log|grep 'deliveryDepth' |awk -F '|' '{if($NF>3 && $NF <= 10 ){sum+=1}} END {print sum}';
awk -F "\t" '{print $3}' dmmspot.log|grep 'deliveryDepth' |awk -F '|' '{if($NF>10 && $NF <= 100 ){sum+=1}} END {print sum}';
awk -F "\t" '{print $3}' dmmspot.log|grep 'deliveryDepth' |awk -F '|' '{if($NF>100 && $NF <= 500 ){sum+=1}} END {print sum}';
awk -F "\t" '{print $3}' dmmspot.log|grep 'deliveryDepth' |awk -F '|' '{if($NF>500 && $NF <= 1000 ){sum+=1}} END {print sum}';
awk -F "\t" '{print $3}' dmmspot.log|grep 'deliveryDepth' |awk -F '|' '{if($NF>1000 ){sum+=1}} END {print sum}';

awk -F "\t" '{print $3}' dmmspot.log|grep 'spotDepth' | wc -l
awk -F "\t" '{print $3}' dmmspot.log|grep 'spotDepth' |awk -F '|' '{if($NF<=3 ){sum+=1}} END {print sum}';
awk -F "\t" '{print $3}' dmmspot.log|grep 'spotDepth' |awk -F '|' '{if($NF>3 && $NF <= 10 ){sum+=1}} END {print sum}';
awk -F "\t" '{print $3}' dmmspot.log|grep 'spotDepth' |awk -F '|' '{if($NF>10 && $NF <= 100 ){sum+=1}} END {print sum}';
awk -F "\t" '{print $3}' dmmspot.log|grep 'spotDepth' |awk -F '|' '{if($NF>100 && $NF <= 500 ){sum+=1}} END {print sum}';
awk -F "\t" '{print $3}' dmmspot.log|grep 'spotDepth' |awk -F '|' '{if($NF>500 && $NF <= 1000 ){sum+=1}} END {print sum}';
awk -F "\t" '{print $3}' dmmspot.log|grep 'spotDepth' |awk -F '|' '{if($NF>1000 ){sum+=1}} END {print sum}';

grep 'placeLimitOrder' * | wc -l
grep 'placeLimitOrder' * | awk -F '|' '{if($NF/10**6 <= 8){sum+=1}} END {print sum}'
grep 'placeLimitOrder' * | awk -F '|' '{if($NF/10**6 > 8 && $NF/10**6 <= 10 ){sum+=1}} END {print sum}';
grep 'placeLimitOrder' * | awk -F '|' '{if($NF/10**6 > 10 && $NF/10**6 <= 20 ){sum+=1}} END {print sum}';
grep 'placeLimitOrder' * | awk -F '|' '{if($NF/10**6 > 20 && $NF/10**6 <= 50 ){sum+=1}} END {print sum}';
grep 'placeLimitOrder' * | awk -F '|' '{if($NF/10**6 > 50 && $NF/10**6 <= 100 ){sum+=1}} END {print sum}';
grep 'placeLimitOrder' * | awk -F '|' '{if($NF/10**6 > 100 ){sum+=1}} END {print sum}';

grep 'cancelByClientId' * | wc -l
grep 'cancelByClientId' * | awk -F '|' '{if($NF/10**6 <= 8){sum+=1}} END {print sum}'
grep 'cancelByClientId' * | awk -F '|' '{if($NF/10**6 > 8 && $NF/10**6 <= 10 ){sum+=1}} END {print sum}';
grep 'cancelByClientId' * | awk -F '|' '{if($NF/10**6 > 10 && $NF/10**6 <= 20 ){sum+=1}} END {print sum}';
grep 'cancelByClientId' * | awk -F '|' '{if($NF/10**6 > 20 && $NF/10**6 <= 50 ){sum+=1}} END {print sum}';
grep 'cancelByClientId' * | awk -F '|' '{if($NF/10**6 > 50 && $NF/10**6 <= 100 ){sum+=1}} END {print sum}';
grep 'cancelByClientId' * | awk -F '|' '{if($NF/10**6 > 100 ){sum+=1}} END {print sum}';

grep 'cancelByOrderId' * | wc -l
grep 'cancelByOrderId' * | awk -F '|' '{if($NF/10**6 <= 8){sum+=1}} END {print sum}'
grep 'cancelByOrderId' * | awk -F '|' '{if($NF/10**6 > 8 && $NF/10**6 <= 10 ){sum+=1}} END {print sum}';
grep 'cancelByOrderId' * | awk -F '|' '{if($NF/10**6 > 10 && $NF/10**6 <= 20 ){sum+=1}} END {print sum}';
grep 'cancelByOrderId' * | awk -F '|' '{if($NF/10**6 > 20 && $NF/10**6 <= 50 ){sum+=1}} END {print sum}';
grep 'cancelByOrderId' * | awk -F '|' '{if($NF/10**6 > 50 && $NF/10**6 <= 100 ){sum+=1}} END {print sum}';
grep 'cancelByOrderId' * | awk -F '|' '{if($NF/10**6 > 100 ){sum+=1}} END {print sum}';

grep 'cancelByAll' * | wc -l
grep 'cancelByAll' * | awk -F '|' '{if($NF/10**6 <= 8){sum+=1}} END {print sum}'
grep 'cancelByAll' * | awk -F '|' '{if($NF/10**6 > 8 && $NF/10**6 <= 10 ){sum+=1}} END {print sum}';
grep 'cancelByAll' * | awk -F '|' '{if($NF/10**6 > 10 && $NF/10**6 <= 20 ){sum+=1}} END {print sum}';
grep 'cancelByAll' * | awk -F '|' '{if($NF/10**6 > 20 && $NF/10**6 <= 50 ){sum+=1}} END {print sum}';
grep 'cancelByAll' * | awk -F '|' '{if($NF/10**6 > 50 && $NF/10**6 <= 100 ){sum+=1}} END {print sum}';
grep 'cancelByAll' * | awk -F '|' '{if($NF/10**6 > 100 ){sum+=1}} END {print sum}';