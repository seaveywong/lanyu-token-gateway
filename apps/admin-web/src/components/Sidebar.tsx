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
}

function Sidebar({ navItems, currentPath, onNavigate, isOpen }: SidebarProps) {
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
    </aside>
  );
}

export default Sidebar;
