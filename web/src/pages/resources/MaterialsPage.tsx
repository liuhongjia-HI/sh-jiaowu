import type { CurrentUser } from '../../types/starline';
import { ContentResourcesPage } from './ContentResourcesPage';
export default function MaterialsPage({ user, courseId, onClearCourse }: { user?: CurrentUser; courseId?: string; onClearCourse?: () => void }) { return <ContentResourcesPage kind="materials" user={user} courseId={courseId} onClearCourse={onClearCourse} />; }
