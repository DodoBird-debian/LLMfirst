// auth.js - Authentication and User Management Logic

(function() {
  const $ = id => document.getElementById(id);

  // Authentication State
  window.AuthState = {
    needsSetup: false,
    authenticated: false,
    user: null // { id, username, role }
  };

  // Check auth state on boot
  window.checkAuth = async function() {
    try {
      const res = await fetch('/api/auth/me');
      const data = await res.json();
      
      window.AuthState.needsSetup = data.needs_setup;
      window.AuthState.authenticated = data.authenticated;
      window.AuthState.user = data.user || null;

      if (window.AuthState.needsSetup) {
        showAuthOverlay(true);
      } else if (!window.AuthState.authenticated) {
        showAuthOverlay(false);
      } else {
        hideAuthOverlay();
      }
    } catch (err) {
      console.error('Failed to verify authentication:', err);
      window.toast('Authentication server connection error', 'error');
    }
  };

  function showAuthOverlay(isSetup) {
    $('auth-overlay').style.display = 'flex';
    if (isSetup) {
      $('auth-title').textContent = 'Create Admin Account';
      $('auth-subtitle').textContent = 'This is LLM WebUI\'s first boot. Please register the primary administrator account.';
      $('btn-auth-submit').textContent = 'Create Admin & Sign In';
    } else {
      $('auth-title').textContent = 'Sign In';
      $('auth-subtitle').textContent = 'Welcome back. Please log in to proceed.';
      $('btn-auth-submit').textContent = 'Sign In';
    }
    // Disable rest of UI interactions
    $('app').style.filter = 'blur(4px)';
    $('app').style.pointerEvents = 'none';
  }

  function hideAuthOverlay() {
    $('auth-overlay').style.display = 'none';
    $('app').style.filter = 'none';
    $('app').style.pointerEvents = 'auto';

    // Update user display details in the top bar
    if (window.AuthState.user) {
      $('user-display-name').textContent = window.AuthState.user.username;
      $('dropdown-username').textContent = window.AuthState.user.username;
      $('dropdown-role').textContent = window.AuthState.user.role.toUpperCase();
      $('dropdown-role').className = 'user-role-badge ' + window.AuthState.user.role;

      // Show/Hide Admin specific controls
      if (window.AuthState.user.role === 'admin') {
        $('btn-admin-panel').style.display = 'block';
        $('shared-key-checkbox-wrapper').style.display = 'block';
      } else {
        $('btn-admin-panel').style.display = 'none';
        $('shared-key-checkbox-wrapper').style.display = 'none';
      }
    }
  }

  // Override fetch to intercept 401 Unauthorized globally
  const originalFetch = window.fetch;
  window.fetch = async function(...args) {
    const response = await originalFetch(...args);
    if (response.status === 401 && !args[0].includes('/api/auth/me') && !args[0].includes('/api/auth/login') && !args[0].includes('/api/auth/setup')) {
      window.AuthState.authenticated = false;
      window.AuthState.user = null;
      showAuthOverlay(false);
      window.toast('Session expired. Please log in again.', 'error');
    }
    return response;
  };

  // Handle Login / Setup Form Submission
  document.addEventListener('DOMContentLoaded', () => {
    const authForm = $('auth-form');
    if (authForm) {
      authForm.addEventListener('submit', async (e) => {
        e.preventDefault();
        const username = $('auth-username').value.trim();
        const password = $('auth-password').value;

        if (!username || !password) {
          window.toast('Username and password are required', 'error');
          return;
        }

        const endpoint = window.AuthState.needsSetup ? '/api/auth/setup' : '/api/auth/login';

        try {
          const res = await fetch(endpoint, {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ username, password })
          });

          if (!res.ok) {
            const errText = await res.text();
            throw new Error(errText || 'Authentication failed');
          }

          // Clear credentials fields
          $('auth-password').value = '';
          window.toast(window.AuthState.needsSetup ? 'Admin account registered successfully!' : 'Signed in successfully!', 'success');

          // Initialize app data on login
          await window.checkAuth();

          // Trigger refresh of main panels if they are defined
          if (window.AuthState.authenticated) {
            if (window.renderConversationList) window.renderConversationList();
            if (window.loadKeys) window.loadKeys();
            if (window.loadModels) window.loadModels();
          }
        } catch (err) {
          window.toast(err.message, 'error');
        }
      });
    }

    // Toggle User Dropdown Menu
    const userMenuBtn = $('btn-user-menu');
    const userDropdown = $('user-dropdown');
    if (userMenuBtn && userDropdown) {
      userMenuBtn.addEventListener('click', (e) => {
        e.stopPropagation();
        userDropdown.style.display = userDropdown.style.display === 'none' ? 'block' : 'none';
      });

      document.addEventListener('click', () => {
        userDropdown.style.display = 'none';
      });

      userDropdown.addEventListener('click', (e) => {
        e.stopPropagation();
      });
    }

    // Handle Log Out
    const logoutBtn = $('btn-logout');
    if (logoutBtn) {
      logoutBtn.addEventListener('click', async () => {
        try {
          await fetch('/api/auth/logout', { method: 'POST' });
          window.toast('Signed out successfully.');
          window.AuthState.authenticated = false;
          window.AuthState.user = null;
          showAuthOverlay(false);
          // Clear active conversation from ui state
          if (window.State) {
            window.State.activeConversationId = null;
            window.State.messages = [];
          }
          if (window.renderConversationList) window.renderConversationList();
          $('welcome').style.display = 'flex';
          $('messages').style.display = 'none';
        } catch (err) {
          window.toast('Failed to sign out', 'error');
        }
      });
    }

    // Handle Admin Modal controls
    const adminBtn = $('btn-admin-panel');
    const adminModal = $('modal-admin');
    const closeAdminBtn = $('btn-close-admin');
    if (adminBtn && adminModal && closeAdminBtn) {
      adminBtn.addEventListener('click', () => {
        adminModal.style.display = 'flex';
        loadAdminUsers();
      });

      closeAdminBtn.addEventListener('click', () => {
        adminModal.style.display = 'none';
      });

      adminModal.addEventListener('click', (e) => {
        if (e.target === adminModal) adminModal.style.display = 'none';
      });
    }

    // Create User account by Admin
    const addUserBtn = $('btn-admin-add-user');
    if (addUserBtn) {
      addUserBtn.addEventListener('click', async () => {
        const usernameInput = $('admin-new-username');
        const passwordInput = $('admin-new-password');
        const username = usernameInput.value.trim();
        const password = passwordInput.value;

        if (!username || !password) {
          window.toast('Username and password are required', 'error');
          return;
        }

        try {
          const res = await fetch('/api/users', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ username, password })
          });

          if (!res.ok) {
            const errText = await res.text();
            throw new Error(errText || 'Failed to create user');
          }

          window.toast(`Account for user ${username} created!`, 'success');
          usernameInput.value = '';
          passwordInput.value = '';
          loadAdminUsers();
        } catch (err) {
          window.toast(err.message, 'error');
        }
      });
    }
  });

  // Load and render user list for admin
  async function loadAdminUsers() {
    const listContainer = $('users-list');
    if (!listContainer) return;
    listContainer.innerHTML = '<div class="loading">Loading accounts…</div>';

    try {
      const res = await fetch('/api/users');
      if (!res.ok) throw new Error('Failed to load users');
      const users = await res.json();

      listContainer.innerHTML = '';
      if (!users || users.length === 0) {
        listContainer.innerHTML = '<div class="empty">No users found</div>';
        return;
      }

      users.forEach(user => {
        const row = document.createElement('div');
        row.className = 'user-row';

        const info = document.createElement('div');
        info.className = 'user-info';
        
        const name = document.createElement('span');
        name.className = 'user-name';
        name.textContent = user.username;

        const badge = document.createElement('span');
        badge.className = 'user-role-badge ' + user.role;
        badge.textContent = user.role;

        info.appendChild(name);
        info.appendChild(badge);

        row.appendChild(info);

        // Don't let admin delete themselves
        if (window.AuthState.user && window.AuthState.user.id !== user.id) {
          const deleteBtn = document.createElement('button');
          deleteBtn.className = 'btn-delete-user';
          deleteBtn.innerHTML = `
            <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
              <polyline points="3 6 5 6 21 6"/>
              <path d="M19 6v14a2 2 0 0 1-2 2H7a2 2 0 0 1-2-2V6m3 0V4a2 2 0 0 1 2-2h4a2 2 0 0 1 2 2v2"/>
            </svg>
          `;
          deleteBtn.title = 'Delete user account';
          deleteBtn.addEventListener('click', async () => {
            if (confirm(`Are you sure you want to delete user "${user.username}"?`)) {
              try {
                const deleteRes = await fetch(`/api/users/${user.id}`, { method: 'DELETE' });
                if (!deleteRes.ok) {
                  const errText = await deleteRes.text();
                  throw new Error(errText || 'Failed to delete user');
                }
                window.toast('User account deleted', 'success');
                loadAdminUsers();
              } catch (err) {
                window.toast(err.message, 'error');
              }
            }
          });
          row.appendChild(deleteBtn);
        }

        listContainer.appendChild(row);
      });
    } catch (err) {
      listContainer.innerHTML = `<div class="error">Error: ${err.message}</div>`;
    }
  }
})();
