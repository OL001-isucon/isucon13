# for gh command
REPO="OL001-isucon/isucon13"
TITLE=$(date -u -d '+9 hours' +"%Y/%m/%d(%a)%H:%M:%S")
ISSUE_URL=$(gh issue create --repo $REPO --title $TITLE --body "")

# for alp command
echo "alp:" > /tmp/alp
echo "\`\`\`" >> /tmp/alp
sudo cat /var/log/nginx/access.log | alp json --config /etc/alp/config.yml >> /tmp/alp
echo "\`\`\`" >> /tmp/alp
gh issue comment $ISSUE_URL --body-file /tmp/alp

# for pt-query-digest command
echo "pt-query-digest:" > /tmp/pt-query-digest
echo "\`\`\`" >> /tmp/pt-query-digest
sudo pt-query-digest /var/log/mysql/mysql-slow.log | head -n 300 >> /tmp/pt-query-digest
echo "\`\`\`" >> /tmp/pt-query-digest
gh issue comment $ISSUE_URL --body-file /tmp/pt-query-digest
