if [ "$GODO_NUM" != "exitCheck" ]; then
   gurl 'name=@姓名' 'sex=@random(男,女)' 'addr=@地址' 'idcard=@身份证' -ugly :2900/person/_doc/@ksuid -raw
fi
