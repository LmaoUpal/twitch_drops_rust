import {Pickaxe} from 'lucide-react';
import {StoreProvider, useStore} from './store';
import {Sidebar} from './components/Sidebar';
import {t} from './i18n';
import Login from './pages/Login';
import Dashboard from './pages/Dashboard';
import Campaigns from './pages/Campaigns';
import AllDrops from './pages/AllDrops';
import Points from './pages/Points';
import Inventory from './pages/Inventory';
import Settings from './pages/Settings';
import Activity from './pages/Activity';

function Shell() {
    const {auth, page, addingAccount, setAddingAccount} = useStore();

    if (auth.state === 'checking') {
        return (
            <div className="login-screen">
                <div className="login-logo"><Pickaxe size={28}/></div>
                <p className="muted" style={{marginTop: 16}}>{t('login.checking')}</p>
            </div>
        );
    }
    if (auth.state !== 'loggedIn') return <Login/>;
    if (addingAccount) return <Login onCancel={() => setAddingAccount(false)}/>;

    return (
        <div className="shell">
            <Sidebar/>
            <main className="main">
                {page === 'dashboard' && <Dashboard/>}
                {page === 'campaigns' && <Campaigns/>}
                {page === 'drops' && <AllDrops/>}
                {page === 'points' && <Points/>}
                {page === 'inventory' && <Inventory/>}
                {page === 'activity' && <Activity/>}
                {page === 'settings' && <Settings/>}
            </main>
        </div>
    );
}

export default function App() {
    return (
        <StoreProvider>
            <Shell/>
        </StoreProvider>
    );
}
