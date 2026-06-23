import styles from './Sidebar.module.css';

interface NavItem {
  path: string;
  label: string;
}

interface SidebarProps {
  navItems: readonly NavItem[];
  currentPath: string;
  onNavigate: (path: string) => void;
  isOpen: boolean;
  userEmail?: string | null;
  onLogout?: () => void;
}

function Sidebar({ navItems, currentPath, onNavigate, isOpen, userEmail, onLogout }: SidebarProps) {
  return (
    <aside className={`${styles.sidebar} ${isOpen ? styles.open : ''}`}>
      <div className={styles.brand}>
        <span className={styles.logo}>🔑</span>
        <span className={styles.brandName}>兰语 Token 网关</span>
      </div>
      <nav className={styles.nav}>
        {navItems.map((item) => (
          <button
            key={item.path}
            className={`${styles.navItem} ${currentPath === item.path ? styles.active : ''}`}
            onClick={() => onNavigate(item.path)}
          >
            {item.label}
          </button>
        ))}
      </nav>
      {userEmail && (
        <div className={styles.userFooter}>
          <span className={styles.userEmail}>{userEmail}</span>
          {onLogout && (
            <button className={styles.logoutButton} onClick={onLogout}>
              退出登录
            </button>
          )}
        </div>
      )}
    </aside>
  );
}

export default Sidebar;
