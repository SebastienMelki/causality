#!/bin/bash

set -e

ADMIN_NAME="${REDASH_ADMIN_NAME:-Causality Admin}"
ADMIN_EMAIL="${REDASH_ADMIN_EMAIL:-admin@causality.local}"
ADMIN_PASSWORD="${REDASH_ADMIN_PASSWORD:-admin123}"

echo "Redash Auto-Setup: Checking for admin user..."

# Wait for database
wait_for_db() {
    echo "Waiting for database to be ready..."
    for i in {1..30}; do
        if python -c "
import os
os.environ.setdefault('REDASH_DATABASE_URL', '$REDASH_DATABASE_URL')
from redash.models import db
from redash import create_app
app = create_app()
with app.app_context():
    db.engine.execute('SELECT 1')
print('Database ready')
" 2>/dev/null; then
            echo "Database is ready!"
            return 0
        fi
        echo "   Attempt $i/30 - waiting for database..."
        sleep 2
    done
    echo "Database not ready after 30 attempts"
    return 1
}

# Check if admin user exists
admin_exists() {
    python -c "
import os
os.environ.setdefault('REDASH_DATABASE_URL', '$REDASH_DATABASE_URL')
from redash.models import User
from redash import create_app
app = create_app()
with app.app_context():
    user = User.query.filter_by(email='$ADMIN_EMAIL').first()
    if user:
        print('EXISTS')
    else:
        print('NOT_EXISTS')
" 2>/dev/null || echo "ERROR"
}

# Create admin user
create_admin() {
    echo "Creating admin user: $ADMIN_NAME ($ADMIN_EMAIL)"
    python manage.py users create_root \
        --org "default" \
        --password "$ADMIN_PASSWORD" \
        "$ADMIN_EMAIL" \
        "$ADMIN_NAME"

    if [ $? -eq 0 ]; then
        echo "Admin user created successfully!"
        return 0
    else
        echo "Failed to create admin user"
        return 1
    fi
}

# Main logic
if wait_for_db; then
    case $(admin_exists) in
        "EXISTS")
            echo "Admin user already exists - skipping setup"
            ;;
        "NOT_EXISTS")
            if create_admin; then
                echo "Redash auto-setup completed!"
            else
                echo "Auto-setup failed - manual setup required at /setup"
            fi
            ;;
        "ERROR")
            echo "Could not check admin user status - manual setup may be required"
            ;;
    esac
else
    echo "Database not ready - skipping auto-setup"
fi

echo "Creating Trino data source..."
python manage.py ds new \
    --org "default" \
    --type "presto" \
    --options '{"host":"trino","port":8080,"catalog":"hive","schema":"causality","username":"redash"}' \
    "Trino Events" 2>/dev/null || echo "Data source already exists or failed"

echo "Redash is ready at http://localhost:5050"
