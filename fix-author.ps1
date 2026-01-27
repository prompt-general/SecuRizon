# Fix git author name and email
cd 'c:\Users\HP\OneDrive\Desktop\Securizon'

# Set git config
git config --global user.name 'caleb rutto'
git config --global user.email 'calebrutto91@gmail.com'

# Verify settings
Write-Host "Git config:"
git config --global user.name
git config --global user.email

# Rewrite history
Write-Host "Rewriting commit history..."
git filter-branch -f --env-filter '
  export GIT_AUTHOR_NAME="caleb rutto"
  export GIT_AUTHOR_EMAIL="calebrutto91@gmail.com"
  export GIT_COMMITTER_NAME="caleb rutto"
  export GIT_COMMITTER_EMAIL="calebrutto91@gmail.com"
' -- --all

# Clean up
Write-Host "Cleaning up..."
Remove-Item -Path .git/refs/original -Recurse -Force 2>$null
git reflog expire --expire=now --all 2>$null
git gc --prune=now --aggressive 2>$null

# Verify
Write-Host "Verifying commits..."
git log --format='%H %an <%ae>' -5

# Force push
Write-Host "Force pushing to origin..."
git push origin master --force-with-lease
