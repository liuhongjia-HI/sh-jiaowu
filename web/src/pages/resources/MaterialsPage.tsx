import type { CurrentUser } from '../../types/starline';
import { ContentResourcesPage } from './ContentResourcesPage';
export default function MaterialsPage({ user }: { user?: CurrentUser }) { return <ContentResourcesPage kind="materials" user={user} />; }
