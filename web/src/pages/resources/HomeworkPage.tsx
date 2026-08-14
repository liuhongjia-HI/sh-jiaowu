import type { CurrentUser } from '../../types/starline';
import { ContentResourcesPage } from './ContentResourcesPage';
export default function HomeworkPage({ user }: { user?: CurrentUser }) { return <ContentResourcesPage kind="homework" user={user} />; }
