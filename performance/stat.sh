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

