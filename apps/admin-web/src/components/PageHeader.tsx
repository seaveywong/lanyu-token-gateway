import styles from './PageHeader.module.css';

interface BreadcrumbItem {
  label: string;
  path?: string;
}

interface PageHeaderProps {
  title: string;
  breadcrumbs?: BreadcrumbItem[];
}

function PageHeader({ title, breadcrumbs }: PageHeaderProps) {
  return (
    <div className={styles.header}>
      {breadcrumbs && breadcrumbs.length > 0 && (
        <nav className={styles.breadcrumb} aria-label="面包屑导航">
          {breadcrumbs.map((item, index) => (
            <span key={index} className={styles.breadcrumbItem}>
              {index > 0 && <span className={styles.separator}>/</span>}
              {item.path ? (
                <a href={item.path} className={styles.breadcrumbLink}>
                  {item.label}
                </a>
              ) : (
                <span className={styles.breadcrumbCurrent}>{item.label}</span>
              )}
            </span>
          ))}
        </nav>
      )}
      <h2 className={styles.title}>{title}</h2>
    </div>
  );
}

export default PageHeader;
